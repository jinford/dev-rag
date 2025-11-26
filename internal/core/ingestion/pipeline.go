package ingestion

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/core/ingestion/chunk"
)

const (
	// DefaultChunkWorkerCount はデフォルトのチャンク分割ワーカー数（CPU バウンド）
	DefaultChunkWorkerCount = 4
	// DefaultEmbeddingWorkerCount はデフォルトのEmbeddingワーカー数（I/O バウンド）
	DefaultEmbeddingWorkerCount = 8
	// DefaultEmbeddingBatchSize はEmbedding APIのデフォルトバッチサイズ
	DefaultEmbeddingBatchSize = 100
	// DefaultFailOnEmbeddingError はEmbeddingエラー時にパイプラインを停止するかのデフォルト値
	DefaultFailOnEmbeddingError = false
	// MinBatchSize は最小バッチサイズ（MaxBatchSize()が0を返した場合のフォールバック）
	MinBatchSize = 1
)

// PipelineConfig はパイプライン処理の設定
type PipelineConfig struct {
	// ChunkWorkerCount はチャンク分割ワーカー数（CPU バウンド処理用）
	ChunkWorkerCount int
	// EmbeddingWorkerCount はEmbedding生成ワーカー数（I/O バウンド処理用）
	EmbeddingWorkerCount int
	// EmbeddingBatchSize はEmbeddingバッチサイズ（Embedder.MaxBatchSize()でクリップされる）
	EmbeddingBatchSize int
	// FailOnEmbeddingError はEmbeddingエラー時にパイプラインを停止するかどうか
	FailOnEmbeddingError bool
}

// DefaultPipelineConfig はデフォルトのパイプライン設定を返す
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		ChunkWorkerCount:     DefaultChunkWorkerCount,
		EmbeddingWorkerCount: DefaultEmbeddingWorkerCount,
		EmbeddingBatchSize:   DefaultEmbeddingBatchSize,
		FailOnEmbeddingError: DefaultFailOnEmbeddingError,
	}
}

// PipelineStats はパイプライン処理の統計情報
type PipelineStats struct {
	ProcessedFiles      int // 正常に処理されたファイル数
	TotalChunks         int // 正常に作成されたチャンク数
	ExpectedChunks      int // チャンク化で生成された期待チャンク数
	FailedFiles         int // 失敗したファイル数
	FailedChunks        int // CreateChunk失敗数
	FailedEmbeddings    int // Embedding生成/保存失敗数
	EmbeddingMismatches int // ベクトル数不一致の回数
}

// documentTask はドキュメント処理タスク
type documentTask struct {
	Document *SourceDocument
	Context  indexDocumentContext
}

// chunkItem はチャンクを Embedding ステージへ受け渡すための最小単位
// （チャンク本体にテキストが含まれるため Text フィールドは不要）
type chunkItem = Chunk

// fileResult はファイル処理の結果
type fileResult struct {
	FilePath       string
	ChunkCount     int // 成功したチャンク数
	ExpectedChunks int // 期待されたチャンク数
	FailedChunks   int // 失敗したチャンク数
	Err            error
}

// IndexPipeline はパイプライン処理を実行する
type IndexPipeline struct {
	repository     Repository
	embedder       Embedder
	chunkerFactory chunk.ChunkerFactory
	languageDetect chunk.LanguageDetector
	config         *PipelineConfig
	logger         *slog.Logger

	// 実際に使用するバッチサイズ（Embedder.MaxBatchSize()でクリップ済み）
	effectiveBatchSize int
}

// NewIndexPipeline は新しいIndexPipelineを作成する
func NewIndexPipeline(
	repository Repository,
	embedder Embedder,
	chunkerFactory chunk.ChunkerFactory,
	languageDetect chunk.LanguageDetector,
	config *PipelineConfig,
	logger *slog.Logger,
) *IndexPipeline {
	if config == nil {
		config = DefaultPipelineConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	// バッチサイズをEmbedderの最大値でクリップ
	effectiveBatchSize := config.EmbeddingBatchSize
	maxBatchSize := embedder.MaxBatchSize()

	// MaxBatchSize が0以下の場合はフォールバック
	if maxBatchSize <= 0 {
		logger.Warn("Embedder.MaxBatchSize()が無効な値を返しました。フォールバック値を使用します",
			"returned", maxBatchSize,
			"fallback", MinBatchSize,
		)
		maxBatchSize = MinBatchSize
	}

	if effectiveBatchSize > maxBatchSize {
		logger.Info("EmbeddingBatchSizeをEmbedderの最大値でクリップ",
			"configured", effectiveBatchSize,
			"max", maxBatchSize,
		)
		effectiveBatchSize = maxBatchSize
	}

	// effectiveBatchSizeも0以下の場合はフォールバック
	if effectiveBatchSize <= 0 {
		effectiveBatchSize = MinBatchSize
	}

	return &IndexPipeline{
		repository:         repository,
		embedder:           embedder,
		chunkerFactory:     chunkerFactory,
		languageDetect:     languageDetect,
		config:             config,
		logger:             logger,
		effectiveBatchSize: effectiveBatchSize,
	}
}

// ProcessDocuments はドキュメントをパイプライン処理でインデックス化する
func (p *IndexPipeline) ProcessDocuments(
	ctx context.Context,
	snapshotID uuid.UUID,
	documents []*SourceDocument,
	docCtx indexDocumentContext,
	shouldIgnore func(*SourceDocument) bool,
) (processedFiles int, totalChunks int, err error) {
	stats, err := p.ProcessDocumentsWithStats(ctx, snapshotID, documents, docCtx, shouldIgnore)
	if err != nil {
		return 0, 0, err
	}
	return stats.ProcessedFiles, stats.TotalChunks, nil
}

// ProcessDocumentsWithStats はドキュメントをパイプライン処理でインデックス化し、詳細な統計を返す
func (p *IndexPipeline) ProcessDocumentsWithStats(
	ctx context.Context,
	snapshotID uuid.UUID,
	documents []*SourceDocument,
	docCtx indexDocumentContext,
	shouldIgnore func(*SourceDocument) bool,
) (*PipelineStats, error) {
	// Stage 1: ドキュメントチャネル（入力）
	docChan := make(chan *documentTask, len(documents))

	// Stage 2: チャンクチャネル（Embedding生成用）
	chunkChan := make(chan *Chunk, p.config.EmbeddingWorkerCount*p.effectiveBatchSize)

	// 結果チャネル
	resultChan := make(chan *fileResult, len(documents))

	// エラー追跡用
	var pipelineErr atomic.Value
	var failedEmbeddings atomic.Int64
	var embeddingMismatches atomic.Int64

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Stage 1: ドキュメントをチャネルに投入
	go func() {
		defer close(docChan)
		for _, doc := range documents {
			if shouldIgnore(doc) {
				p.logger.Debug("ドキュメントを除外", "path", doc.Path)
				continue
			}
			select {
			case docChan <- &documentTask{Document: doc, Context: docCtx}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Stage 2: チャンク分割ワーカー（ファイル→チャンク→バッチ送信）
	var chunkWg sync.WaitGroup
	chunkWg.Add(p.config.ChunkWorkerCount)
	for i := 0; i < p.config.ChunkWorkerCount; i++ {
		go func() {
			defer chunkWg.Done()
			p.chunkWorker(ctx, snapshotID, docChan, chunkChan, resultChan)
		}()
	}

	// チャンク分割完了を待ってチャンクチャネルを閉じる
	go func() {
		chunkWg.Wait()
		close(chunkChan)
	}()

	// Stage 3: Embedding生成・保存ワーカー
	var embeddingWg sync.WaitGroup
	embeddingWg.Add(p.config.EmbeddingWorkerCount)
	for i := 0; i < p.config.EmbeddingWorkerCount; i++ {
		go func() {
			defer embeddingWg.Done()
			p.embeddingWorker(ctx, cancel, chunkChan, &pipelineErr, &failedEmbeddings, &embeddingMismatches)
		}()
	}

	// Embedding完了を待って結果チャネルを閉じる
	go func() {
		embeddingWg.Wait()
		close(resultChan)
	}()

	// 結果集計
	stats := &PipelineStats{}
	for result := range resultChan {
		if result.Err != nil {
			p.logger.Warn("ドキュメントのインデックス化に失敗",
				"path", result.FilePath,
				"error", result.Err,
			)
			stats.FailedFiles++
			continue
		}
		stats.ProcessedFiles++
		stats.TotalChunks += result.ChunkCount
		stats.ExpectedChunks += result.ExpectedChunks
		stats.FailedChunks += result.FailedChunks
	}

	stats.FailedEmbeddings = int(failedEmbeddings.Load())
	stats.EmbeddingMismatches = int(embeddingMismatches.Load())

	// 致命的エラーがあった場合
	if errVal := pipelineErr.Load(); errVal != nil {
		if pipeErr, ok := errVal.(error); ok {
			return stats, fmt.Errorf("パイプライン処理中に致命的エラー: %w", pipeErr)
		}
	}

	// 統計情報をログ出力
	if stats.FailedFiles > 0 || stats.FailedChunks > 0 || stats.FailedEmbeddings > 0 || stats.EmbeddingMismatches > 0 {
		p.logger.Warn("パイプライン処理完了（一部失敗あり）",
			"processedFiles", stats.ProcessedFiles,
			"totalChunks", stats.TotalChunks,
			"expectedChunks", stats.ExpectedChunks,
			"failedFiles", stats.FailedFiles,
			"failedChunks", stats.FailedChunks,
			"failedEmbeddings", stats.FailedEmbeddings,
			"embeddingMismatches", stats.EmbeddingMismatches,
		)
	}

	return stats, nil
}

// chunkWorker はドキュメントをチャンク分割し、バッチを送信するワーカー
func (p *IndexPipeline) chunkWorker(
	ctx context.Context,
	snapshotID uuid.UUID,
	docChan <-chan *documentTask,
	chunkChan chan<- *Chunk,
	resultChan chan<- *fileResult,
) {
	for task := range docChan {
		select {
		case <-ctx.Done():
			return
		default:
		}

		doc := task.Document

		// 言語を検出
		language, err := p.languageDetect.DetectLanguage(doc.Path, []byte(doc.Content))
		if err != nil {
			p.logger.Debug("言語検出に失敗、デフォルト処理を続行",
				"path", doc.Path,
				"error", err,
			)
			language = "unknown"
		}

		// ファイルを作成
		file, err := p.repository.CreateFile(
			ctx,
			snapshotID,
			doc.Path,
			doc.Size,
			"text/plain",
			doc.ContentHash,
			&language,
			nil,
		)
		if err != nil {
			p.logger.Warn("ファイルの作成に失敗",
				"path", doc.Path,
				"error", err,
			)
			select {
			case resultChan <- &fileResult{FilePath: doc.Path, Err: err}:
			case <-ctx.Done():
			}
			continue
		}

		// チャンカーを取得
		chunker, err := p.chunkerFactory.GetChunker(language)
		if err != nil {
			p.logger.Warn("チャンカーの取得に失敗",
				"path", doc.Path,
				"error", err,
			)
			select {
			case resultChan <- &fileResult{FilePath: doc.Path, Err: err}:
			case <-ctx.Done():
			}
			continue
		}

		// チャンク化
		chunkResults, err := chunker.Chunk(ctx, doc.Path, doc.Content)
		if err != nil {
			p.logger.Warn("チャンク化に失敗",
				"path", doc.Path,
				"error", err,
			)
			select {
			case resultChan <- &fileResult{FilePath: doc.Path, Err: err}:
			case <-ctx.Done():
			}
			continue
		}

		expectedChunks := len(chunkResults)
		fileChunkCount := 0
		failedChunkCount := 0

		chunkInputs := make([]*Chunk, 0, len(chunkResults))
		for i, result := range chunkResults {
			metadata := convertChunkMetadata(result.Metadata)
			chunkKey := generateChunkKey(task.Context, doc.Path, result.StartLine, result.EndLine, i)
			metadata.ChunkKey = chunkKey

			chunkInputs = append(chunkInputs, &Chunk{
				ID:                   uuid.New(),
				FileID:               file.ID,
				Ordinal:              i,
				StartLine:            result.StartLine,
				EndLine:              result.EndLine,
				Content:              result.Content,
				ContentHash:          computeContentHash(result.Content),
				TokenCount:           result.Tokens,
				Type:                 metadata.Type,
				Name:                 metadata.Name,
				ParentName:           metadata.ParentName,
				Signature:            metadata.Signature,
				DocComment:           metadata.DocComment,
				Imports:              metadata.Imports,
				Calls:                metadata.Calls,
				LinesOfCode:          metadata.LinesOfCode,
				CommentRatio:         metadata.CommentRatio,
				CyclomaticComplexity: metadata.CyclomaticComplexity,
				EmbeddingContext:     metadata.EmbeddingContext,
				Level:                metadata.Level,
				ImportanceScore:      metadata.ImportanceScore,
				StandardImports:      metadata.StandardImports,
				ExternalImports:      metadata.ExternalImports,
				InternalCalls:        metadata.InternalCalls,
				ExternalCalls:        metadata.ExternalCalls,
				TypeDependencies:     metadata.TypeDependencies,
				SourceSnapshotID:     metadata.SourceSnapshotID,
				GitCommitHash:        metadata.GitCommitHash,
				Author:               metadata.Author,
				UpdatedAt:            metadata.UpdatedAt,
				FileVersion:          metadata.FileVersion,
				IsLatest:             metadata.IsLatest,
				ChunkKey:             metadata.ChunkKey,
			})
		}

		// バッチ作成
		if err := p.repository.BatchCreateChunks(ctx, chunkInputs); err != nil {
			p.logger.Warn("チャンクのバッチ作成に失敗",
				"path", doc.Path,
				"error", err,
			)
			failedChunkCount = len(chunkResults)
			select {
			case resultChan <- &fileResult{FilePath: doc.Path, Err: err}:
			case <-ctx.Done():
			}
			continue
		}

		// 生成済み ID をそのまま Embedding 側へ送る
		for _, ch := range chunkInputs {
			select {
			case chunkChan <- ch:
			case <-ctx.Done():
				return
			}
			fileChunkCount++
		}

		// ファイル処理完了を通知
		select {
		case resultChan <- &fileResult{
			FilePath:       doc.Path,
			ChunkCount:     fileChunkCount,
			ExpectedChunks: expectedChunks,
			FailedChunks:   failedChunkCount,
		}:
		case <-ctx.Done():
			return
		}
	}
}

// embeddingWorker はバッチのEmbeddingを生成して保存するワーカー
func (p *IndexPipeline) embeddingWorker(
	ctx context.Context,
	cancel context.CancelFunc,
	chunkChan <-chan *Chunk,
	pipelineErr *atomic.Value,
	failedEmbeddings *atomic.Int64,
	embeddingMismatches *atomic.Int64,
) {
	// Chunk のみを保持（テキストは chunk.Content を利用）
	pendingItems := make([]*Chunk, 0, p.effectiveBatchSize)

	processBatch := func() bool {
		if len(pendingItems) == 0 {
			return true
		}

		texts := make([]string, 0, len(pendingItems))
		for _, it := range pendingItems {
			texts = append(texts, it.Content)
		}

		vectors, err := p.embedder.BatchEmbed(ctx, texts)
		if err != nil {
			p.logger.Error("バッチEmbedding生成に失敗",
				"batchSize", len(texts),
				"error", err,
			)
			failedEmbeddings.Add(int64(len(pendingItems)))

			if p.config.FailOnEmbeddingError {
				pipelineErr.Store(fmt.Errorf("embedding生成失敗: %w", err))
				cancel()
				return false
			}
			pendingItems = pendingItems[:0]
			return true
		}

		if len(vectors) != len(pendingItems) {
			p.logger.Error("Embeddingベクトル数が不一致",
				"expected", len(pendingItems),
				"actual", len(vectors),
			)
			embeddingMismatches.Add(1)

			diff := len(vectors) - len(pendingItems)
			if diff < 0 {
				diff = -diff
			}
			failedEmbeddings.Add(int64(diff))

			if p.config.FailOnEmbeddingError {
				pipelineErr.Store(errors.New("Embeddingベクトル数が入力と一致しません"))
				cancel()
				return false
			}
		}

		limit := min(len(vectors), len(pendingItems))
		embeddings := make([]*Embedding, 0, limit)
		for i := range limit {
			embeddings = append(embeddings, &Embedding{
				ChunkID: pendingItems[i].ID,
				Vector:  vectors[i],
				Model:   p.embedder.ModelName(),
			})
		}

		if err := p.repository.BatchCreateEmbeddings(ctx, embeddings); err != nil {
			p.logger.Error("バッチembedding保存に失敗",
				"count", len(embeddings),
				"error", err,
			)
			failedEmbeddings.Add(int64(len(embeddings)))

			if p.config.FailOnEmbeddingError {
				pipelineErr.Store(fmt.Errorf("embedding保存失敗: %w", err))
				cancel()
				return false
			}
		}

		pendingItems = pendingItems[:0]
		return true
	}

	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-chunkChan:
			if !ok {
				processBatch()
				return
			}

			pendingItems = append(pendingItems, item)

			if len(pendingItems) >= p.effectiveBatchSize {
				if !processBatch() {
					return
				}
			}
		}
	}
}

// convertChunkMetadata は chunk.ChunkMetadata を ingestion.ChunkMetadata に変換する。
func convertChunkMetadata(meta *chunk.ChunkMetadata) *ChunkMetadata {
	return &ChunkMetadata{
		Type:                 meta.Type,
		Name:                 meta.Name,
		ParentName:           meta.ParentName,
		Signature:            meta.Signature,
		DocComment:           meta.DocComment,
		Imports:              meta.Imports,
		Calls:                meta.Calls,
		LinesOfCode:          meta.LinesOfCode,
		CommentRatio:         meta.CommentRatio,
		CyclomaticComplexity: meta.CyclomaticComplexity,
		EmbeddingContext:     meta.EmbeddingContext,
		Level:                meta.Level,
		ImportanceScore:      meta.ImportanceScore,
		StandardImports:      meta.StandardImports,
		ExternalImports:      meta.ExternalImports,
		InternalCalls:        meta.InternalCalls,
		ExternalCalls:        meta.ExternalCalls,
		TypeDependencies:     meta.TypeDependencies,
		SourceSnapshotID:     meta.SourceSnapshotID,
		GitCommitHash:        meta.GitCommitHash,
		Author:               meta.Author,
		UpdatedAt:            nil,
		FileVersion:          meta.FileVersion,
		IsLatest:             meta.IsLatest,
		ChunkKey:             meta.ChunkKey,
	}
}

// computeContentHash はコンテンツのSHA256ハッシュを計算する
func computeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// Chunker から返されるメタデータは non-nil である前提（Chunker 実装側で保証）
