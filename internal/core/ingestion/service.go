package ingestion

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/core/ingestion/chunk"
	"github.com/jinford/dev-rag/internal/core/wiki"
)

// IndexResult はインデックス化処理の結果を表す
type IndexResult struct {
	SnapshotID        uuid.UUID
	VersionIdentifier string
	ProcessedFiles    int
	TotalChunks       int
	Duration          time.Duration
}

// IndexService はインデックス化のユースケースを提供する
type IndexService struct {
	repository      Repository
	sourceProvider  SourceProvider
	embedder        Embedder
	llmClient       wiki.LLMClient // オプショナル
	chunkerFactory  chunk.ChunkerFactory
	languageDetect  chunk.LanguageDetector
	tokenCounter    chunk.TokenCounter
	chunkerConfig   *chunk.ChunkerConfig
	logger          *slog.Logger
}

// IndexServiceConfig はIndexServiceの設定
type IndexServiceConfig struct {
	Repository      Repository
	SourceProvider  SourceProvider
	Embedder        Embedder
	LLMClient       wiki.LLMClient // オプショナル
	ChunkerFactory  chunk.ChunkerFactory
	LanguageDetect  chunk.LanguageDetector
	TokenCounter    chunk.TokenCounter
	ChunkerConfig   *chunk.ChunkerConfig
	Logger          *slog.Logger
}

// NewIndexService は新しいIndexServiceを作成する
func NewIndexService(cfg IndexServiceConfig) *IndexService {
	if cfg.ChunkerConfig == nil {
		cfg.ChunkerConfig = chunk.DefaultChunkerConfig()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &IndexService{
		repository:     cfg.Repository,
		sourceProvider: cfg.SourceProvider,
		embedder:       cfg.Embedder,
		llmClient:      cfg.LLMClient,
		chunkerFactory: cfg.ChunkerFactory,
		languageDetect: cfg.LanguageDetect,
		tokenCounter:   cfg.TokenCounter,
		chunkerConfig:  cfg.ChunkerConfig,
		logger:         cfg.Logger,
	}
}

// IndexSource はソースをインデックス化する
func (s *IndexService) IndexSource(ctx context.Context, params IndexParams) (*IndexResult, error) {
	startTime := time.Now()

	s.logger.Info("インデックス化を開始",
		"sourceType", s.sourceProvider.GetSourceType(),
		"identifier", params.Identifier,
		"product", params.ProductName,
		"forceInit", params.ForceInit,
	)

	// パラメータのバリデーション
	if err := s.validateParams(params); err != nil {
		return nil, fmt.Errorf("パラメータのバリデーションエラー: %w", err)
	}

	// Product を取得または作成
	product, err := s.repository.CreateProductIfNotExists(ctx, params.ProductName, nil)
	if err != nil {
		return nil, fmt.Errorf("プロダクトの取得/作成に失敗: %w", err)
	}

	// Source を取得または作成
	sourceName := s.sourceProvider.ExtractSourceName(params.Identifier)
	sourceMetadata := s.sourceProvider.CreateMetadata(params)
	source, err := s.repository.CreateSourceIfNotExists(
		ctx,
		sourceName,
		s.sourceProvider.GetSourceType(),
		product.ID,
		sourceMetadata,
	)
	if err != nil {
		return nil, fmt.Errorf("ソースの取得/作成に失敗: %w", err)
	}

	// ソースからドキュメントを取得
	documents, versionIdentifier, err := s.sourceProvider.FetchDocuments(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("ドキュメントの取得に失敗: %w", err)
	}

	s.logger.Info("ドキュメントを取得",
		"count", len(documents),
		"version", versionIdentifier,
	)

	// 既存のスナップショットをチェック
	if !params.ForceInit {
		existingSnapshot, err := s.repository.GetSnapshotByVersion(ctx, source.ID, versionIdentifier)
		if err == nil && existingSnapshot != nil && existingSnapshot.Indexed {
			s.logger.Info("既にインデックス済みのバージョン",
				"snapshotID", existingSnapshot.ID,
				"version", versionIdentifier,
			)
			return &IndexResult{
				SnapshotID:        existingSnapshot.ID,
				VersionIdentifier: versionIdentifier,
				ProcessedFiles:    0,
				TotalChunks:       0,
				Duration:          time.Since(startTime),
			}, nil
		}
	}

	// 新しいスナップショットを作成
	snapshot, err := s.repository.CreateSnapshot(ctx, source.ID, versionIdentifier)
	if err != nil {
		// 重複エラーの場合、既存スナップショットを取得して再利用
		if errors.Is(err, ErrSnapshotVersionConflict) {
			s.logger.Info("スナップショットが既に存在します。既存のスナップショットを再利用",
				"version", versionIdentifier,
			)
			existingSnapshot, getErr := s.repository.GetSnapshotByVersion(ctx, source.ID, versionIdentifier)
			if getErr != nil {
				return nil, fmt.Errorf("既存スナップショットの取得に失敗: %w", getErr)
			}
			// 既にインデックス済みの場合はそのまま返す
			if existingSnapshot.Indexed {
				return &IndexResult{
					SnapshotID:        existingSnapshot.ID,
					VersionIdentifier: versionIdentifier,
					ProcessedFiles:    0,
					TotalChunks:       0,
					Duration:          time.Since(startTime),
				}, nil
			}
			// インデックス未完了の場合は再利用してインデックス処理を継続
			snapshot = existingSnapshot
		} else {
			return nil, fmt.Errorf("スナップショットの作成に失敗: %w", err)
		}
	}

	// ドキュメントをインデックス化
	processedFiles := 0
	totalChunks := 0

	// インデックス化コンテキストを作成
	docCtx := indexDocumentContext{
		ProductName:       params.ProductName,
		SourceName:        sourceName,
		VersionIdentifier: versionIdentifier,
	}

	for _, doc := range documents {
		// 除外すべきドキュメントをスキップ
		if s.sourceProvider.ShouldIgnore(doc) {
			s.logger.Debug("ドキュメントを除外", "path", doc.Path)
			continue
		}

		// ファイルをインデックス化
		chunks, err := s.indexDocument(ctx, snapshot.ID, doc, docCtx)
		if err != nil {
			s.logger.Warn("ドキュメントのインデックス化に失敗",
				"path", doc.Path,
				"error", err,
			)
			continue
		}

		processedFiles++
		totalChunks += len(chunks)
	}

	// スナップショットを完了としてマーク
	if err := s.repository.MarkSnapshotIndexed(ctx, snapshot.ID); err != nil {
		return nil, fmt.Errorf("スナップショットのマークに失敗: %w", err)
	}

	duration := time.Since(startTime)

	s.logger.Info("インデックス化が完了",
		"snapshotID", snapshot.ID,
		"processedFiles", processedFiles,
		"totalChunks", totalChunks,
		"duration", duration,
	)

	return &IndexResult{
		SnapshotID:        snapshot.ID,
		VersionIdentifier: versionIdentifier,
		ProcessedFiles:    processedFiles,
		TotalChunks:       totalChunks,
		Duration:          duration,
	}, nil
}

// indexDocument は単一のドキュメントをインデックス化する
func (s *IndexService) indexDocument(ctx context.Context, snapshotID uuid.UUID, doc *SourceDocument, docCtx indexDocumentContext) ([]*Chunk, error) {
	// 言語を検出
	language, err := s.languageDetect.DetectLanguage(doc.Path, []byte(doc.Content))
	if err != nil {
		s.logger.Debug("言語検出に失敗、デフォルト処理を続行",
			"path", doc.Path,
			"error", err,
		)
		language = "unknown"
	}

	// ファイルを作成
	file, err := s.repository.CreateFile(
		ctx,
		snapshotID,
		doc.Path,
		doc.Size,
		"text/plain", // TODO: content typeの適切な検出
		doc.ContentHash,
		&language,
		nil, // domain
	)
	if err != nil {
		return nil, fmt.Errorf("ファイルの作成に失敗: %w", err)
	}

	// チャンカーを取得
	chkr, err := s.chunkerFactory.GetChunker(language)
	if err != nil {
		return nil, fmt.Errorf("チャンカーの取得に失敗: %w", err)
	}

	// チャンク化
	chunkResults, err := chkr.Chunk(ctx, doc.Path, doc.Content)
	if err != nil {
		return nil, fmt.Errorf("チャンク化に失敗: %w", err)
	}

	// チャンクを保存
	chunks := make([]*Chunk, 0, len(chunkResults))
	embeddings := make([]*Embedding, 0, len(chunkResults))

	for i, result := range chunkResults {
		// チャンクメタデータを変換
		metadata := s.convertChunkMetadata(result.Metadata)

		// ChunkKeyを生成
		chunkKey := generateChunkKey(docCtx, doc.Path, result.StartLine, result.EndLine, i)
		metadata.ChunkKey = chunkKey

		// チャンクを作成
		chunk, err := s.repository.CreateChunk(
			ctx,
			file.ID,
			i,
			result.StartLine,
			result.EndLine,
			result.Content,
			s.computeContentHash(result.Content),
			result.Tokens,
			metadata,
		)
		if err != nil {
			s.logger.Warn("チャンクの作成に失敗",
				"path", doc.Path,
				"ordinal", i,
				"error", err,
			)
			continue
		}

		chunks = append(chunks, chunk)

		// Embeddingを生成
		vector, err := s.embedder.Embed(ctx, result.Content)
		if err != nil {
			s.logger.Warn("Embeddingの生成に失敗",
				"chunkID", chunk.ID,
				"error", err,
			)
			continue
		}

		embeddings = append(embeddings, &Embedding{
			ChunkID: chunk.ID,
			Vector:  vector,
			Model:   s.embedder.ModelName(),
		})
	}

	// Embeddingをバッチで保存
	if len(embeddings) > 0 {
		if err := s.repository.BatchCreateEmbeddings(ctx, embeddings); err != nil {
			return nil, fmt.Errorf("Embeddingのバッチ作成に失敗: %w", err)
		}
	}

	return chunks, nil
}

// validateParams はインデックス化パラメータをバリデートする
func (s *IndexService) validateParams(params IndexParams) error {
	if params.Identifier == "" {
		return fmt.Errorf("identifier は必須です")
	}
	if params.ProductName == "" {
		return fmt.Errorf("product name は必須です")
	}
	return nil
}

// convertChunkMetadata は chunk.ChunkMetadata を ingestion.ChunkMetadata に変換する
func (s *IndexService) convertChunkMetadata(meta *chunk.ChunkMetadata) *ChunkMetadata {
	if meta == nil {
		return nil
	}

	// chunk.ChunkMetadata を ingestion.ChunkMetadata に変換
	// chunk.ChunkMetadata には UpdatedAt がないため、nil を設定
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
		UpdatedAt:            nil, // chunk.ChunkMetadata にはこのフィールドがない
		FileVersion:          meta.FileVersion,
		IsLatest:             meta.IsLatest,
		ChunkKey:             meta.ChunkKey,
	}
}

// computeContentHash はコンテンツのSHA256ハッシュを計算する
func (s *IndexService) computeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// indexDocumentContext はドキュメントインデックス化のコンテキスト情報
type indexDocumentContext struct {
	ProductName       string
	SourceName        string
	VersionIdentifier string // commit hash や version など
}

// generateChunkKey はチャンクのユニークキーを生成する
// 形式: {product_name}/{source_name}/{file_path}#L{start}-L{end}:{ordinal}@{commit_hash}
func generateChunkKey(ctx indexDocumentContext, filePath string, startLine, endLine, ordinal int) string {
	return fmt.Sprintf("%s/%s/%s#L%d-L%d:%d@%s",
		ctx.ProductName,
		ctx.SourceName,
		filePath,
		startLine,
		endLine,
		ordinal,
		ctx.VersionIdentifier,
	)
}
