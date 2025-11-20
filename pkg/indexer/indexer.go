package indexer

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-enry/go-enry/v2"
	"github.com/google/uuid"

	"github.com/jinford/dev-rag/pkg/indexer/chunker"
	"github.com/jinford/dev-rag/pkg/indexer/detector"
	embedpkg "github.com/jinford/dev-rag/pkg/indexer/embedder"
	"github.com/jinford/dev-rag/pkg/indexer/provider"
	"github.com/jinford/dev-rag/pkg/lock"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
	"github.com/jinford/dev-rag/pkg/repository/txprovider"
)

// Indexer はソースのインデックス化を管理します
type Indexer struct {
	sourceReadRepo *repository.SourceRepositoryR
	indexReadRepo  *repository.IndexRepositoryR
	txProvider     *txprovider.TransactionProvider
	srcProviders   map[models.SourceType]provider.SourceProvider
	chunker        *chunker.Chunker
	embedder       *embedpkg.Embedder
	contextBuilder *embedpkg.ContextBuilder
	detector       *detector.ContentTypeDetector
	logger         *slog.Logger
	metrics        *IndexMetrics // Phase 1追加: メトリクス収集
}

// NewIndexer は新しいIndexerを作成します
func NewIndexer(
	sourceRepo *repository.SourceRepositoryR,
	indexRepo *repository.IndexRepositoryR,
	txProvider *txprovider.TransactionProvider,
	chunker *chunker.Chunker,
	embedder *embedpkg.Embedder,
	detector *detector.ContentTypeDetector,
	logger *slog.Logger,
) (*Indexer, error) {
	// ContextBuilderを初期化
	contextBuilder, err := embedpkg.NewContextBuilder()
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}

	return &Indexer{
		sourceReadRepo: sourceRepo,
		indexReadRepo:  indexRepo,
		txProvider:     txProvider,
		srcProviders:   make(map[models.SourceType]provider.SourceProvider),
		chunker:        chunker,
		embedder:       embedder,
		contextBuilder: contextBuilder,
		detector:       detector,
		logger:         logger,
		metrics:        NewIndexMetrics(), // Phase 1追加: メトリクス初期化
	}, nil
}

// RegisterProvider はソースプロバイダーを登録します
func (idx *Indexer) RegisterProvider(srcProvider provider.SourceProvider) {
	idx.srcProviders[srcProvider.GetSourceType()] = srcProvider
}

// IndexResult はインデックス化の結果
type IndexResult struct {
	SnapshotID        string
	VersionIdentifier string
	ProcessedFiles    int
	TotalChunks       int
	Duration          time.Duration
}

// IndexSource は指定されたソースタイプのソースをインデックス化します（共通処理）
func (idx *Indexer) IndexSource(ctx context.Context, sourceType models.SourceType, params provider.IndexParams) (*IndexResult, error) {
	// ソースプロバイダーを取得
	prov, ok := idx.srcProviders[sourceType]
	if !ok {
		return nil, fmt.Errorf("provider for source type %s is not registered", sourceType)
	}

	startTime := time.Now()
	// メトリクスをリセット
	idx.metrics = NewIndexMetrics()

	idx.logger.Info("Starting index process",
		"sourceType", prov.GetSourceType(),
		"identifier", params.Identifier,
		"product", params.ProductName,
		"forceInit", params.ForceInit,
	)

	// ドキュメント一覧を取得
	documents, versionIdentifier, err := prov.FetchDocuments(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch documents: %w", err)
	}

	// プロダクト/ソースを短期Txで確定
	source, err := idx.ensureSource(ctx, prov, params)
	if err != nil {
		return nil, err
	}

	// 差分判定に必要な前回スナップショットとハッシュ群を取得
	previousSnapshot, previousFileHashes, err := idx.loadPreviousSnapshot(ctx, source, params.ForceInit)
	if err != nil {
		return nil, err
	}

	// Source名を抽出（chunk_key生成用）
	sourceName := prov.ExtractSourceName(params.Identifier)

	// 非トランザクションでチャンク化・Embedding 済みデータを構築
	preparedDocs, err := idx.prepareDocuments(ctx, prov, documents, previousFileHashes, params.ProductName, sourceName)
	if err != nil {
		return nil, err
	}

	// 前回スナップショットから削除されたパスを抽出
	deletedPaths := detectDeletedPaths(previousFileHashes, preparedDocs.currentDocPaths)

	// アドバイザリロックIDを生成
	lockID := lock.GenerateLockID(string(prov.GetSourceType()), params.Identifier)

	result, err := idx.commitPreparedDocuments(ctx, &commitPreparedDocumentParams{
		lockID:            lockID,
		source:            source,
		versionIdentifier: versionIdentifier,
		previousSnapshot:  previousSnapshot,
		deletedPaths:      deletedPaths,
		preparedFiles:     preparedDocs.files,
		processedFiles:    preparedDocs.processedFiles,
		totalChunks:       preparedDocs.totalChunks,
		startTime:         startTime,
	})
	if err != nil {
		return nil, err
	}

	// メトリクスをログ出力
	idx.logMetrics()

	idx.logger.Info("Index process completed",
		"snapshotID", result.SnapshotID,
		"processedFiles", result.ProcessedFiles,
		"totalChunks", result.TotalChunks,
		"duration", result.Duration,
	)

	return result, nil
}

// preparedDocumentsResult は事前処理したドキュメント群を保持します
type preparedDocumentsResult struct {
	files           []*preparedFile
	processedFiles  int
	totalChunks     int
	currentDocPaths map[string]bool
}

type preparedFile struct {
	Path        string
	Size        int64
	ContentType string
	ContentHash string
	Chunks      []*preparedChunk

	// Phase 1追加: コミットメタデータ
	CommitHash string
	Author     string
	UpdatedAt  time.Time

	// Phase 1追加: chunk_key生成用
	ProductName string
	SourceName  string

	// Phase 1追加: 言語とドメイン
	Language *string
	Domain   *string
}

type preparedChunk struct {
	StartLine int
	EndLine   int
	Content   string
	Tokens    int
	Hash      string
	Embedding []float32
	Metadata  *repository.ChunkMetadata // Phase 1追加
}

// commitPreparedDocumentParams は書き込み処理に必要な情報をまとめます
type commitPreparedDocumentParams struct {
	lockID            int64
	source            *models.Source
	versionIdentifier string
	previousSnapshot  *models.SourceSnapshot
	deletedPaths      []string
	preparedFiles     []*preparedFile
	processedFiles    int
	totalChunks       int
	startTime         time.Time
}

// loadPreviousSnapshot は差分判定用に最新インデックス済みスナップショットとファイルハッシュを読み出します
func (idx *Indexer) loadPreviousSnapshot(ctx context.Context, source *models.Source, forceInit bool) (*models.SourceSnapshot, map[string]string, error) {
	if forceInit {
		// 初回インデックスでは比較対象を空集合にする
		return nil, make(map[string]string), nil
	}

	// 最新のインデックス済みスナップショットを取得
	latestSnapshot, err := idx.sourceReadRepo.GetLatestIndexedSnapshot(ctx, source.ID)
	if err != nil {
		// スナップショットが存在しない場合は初回インデックスとして扱う
		if errors.Is(err, repository.ErrNotFound) {
			return nil, make(map[string]string), nil
		}
		return nil, nil, fmt.Errorf("failed to get latest indexed snapshot: %w", err)
	}

	// スナップショット配下のファイルハッシュを取得
	hashes, err := idx.indexReadRepo.GetFileHashesBySnapshot(ctx, latestSnapshot.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get previous file hashes: %w", err)
	}

	return latestSnapshot, hashes, nil
}

// ensureSource はプロダクト/ソースをUpsertする短いトランザクションを実行します
func (idx *Indexer) ensureSource(ctx context.Context, prov provider.SourceProvider, params provider.IndexParams) (*models.Source, error) {
	return txprovider.Transact(ctx, idx.txProvider, func(adapters *txprovider.Adapter) (*models.Source, error) {
		// プロダクトを存在しなければ作成
		product, err := adapters.Products.CreateIfNotExists(ctx, params.ProductName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create product: %w", err)
		}
		productID := repository.PgtypeToUUID(product.ID)

		// ソース名とメタデータを構築して upsert
		sourceName := prov.ExtractSourceName(params.Identifier)
		metadata := prov.CreateMetadata(params)

		source, err := adapters.Sources.CreateIfNotExists(ctx, sourceName, prov.GetSourceType(), productID, metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to create source: %w", err)
		}

		return source, nil
	})
}

// prepareDocuments はチャンク化・Embedding 済みのドキュメント群を構築します
func (idx *Indexer) prepareDocuments(ctx context.Context, prov provider.SourceProvider, documents []*provider.SourceDocument, previousFileHashes map[string]string, productName, sourceName string) (*preparedDocumentsResult, error) {
	prepared := &preparedDocumentsResult{
		files:           make([]*preparedFile, 0, len(documents)),
		currentDocPaths: make(map[string]bool, len(documents)),
	}

	for _, doc := range documents {
		// 取得できたドキュメントのパスを記録
		prepared.currentDocPaths[doc.Path] = true

		// 除外設定に合致すればスキップ
		if prov.ShouldIgnore(doc) {
			idx.logger.Debug("Skipping ignored document", "path", doc.Path)
			continue
		}

		// 前回とハッシュが同一ならスキップ
		if previousHash, exists := previousFileHashes[doc.Path]; exists && previousHash == doc.ContentHash {
			idx.logger.Debug("Skipping unchanged document", "path", doc.Path)
			continue
		}

		// コンテンツタイプに応じてチャンク戦略を選択
		contentType := idx.detector.DetectContentType(doc.Path, []byte(doc.Content))

		// 言語検出とドメイン分類を実行
		language := idx.detectLanguage(doc.Path, doc.Content)
		domain := idx.classifyDomain(doc.Path)

		// コンテンツをチャンク化（メタデータ付き、メトリクス収集）
		chunksWithMeta, err := idx.chunker.ChunkWithMetadataAndMetrics(doc.Content, contentType, idx.metrics, idx.logger)
		if err != nil {
			idx.logger.Warn("Failed to chunk document", "path", doc.Path, "error", err)
			continue
		}

		if len(chunksWithMeta) == 0 {
			idx.logger.Debug("No chunks generated", "path", doc.Path)
			continue
		}

		// Embedding を含んだチャンク構造を準備
		chunkPayloads, err := idx.prepareChunks(ctx, chunksWithMeta, doc.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare chunks for file %s: %w", doc.Path, err)
		}

		prepared.files = append(prepared.files, &preparedFile{
			Path:        doc.Path,
			Size:        doc.Size,
			ContentType: contentType,
			ContentHash: doc.ContentHash,
			Chunks:      chunkPayloads,
			// Phase 1追加: コミットメタデータを保持
			CommitHash:  doc.CommitHash,
			Author:      doc.Author,
			UpdatedAt:   doc.UpdatedAt,
			// Phase 1追加: chunk_key生成用の情報を保持
			ProductName: productName,
			SourceName:  sourceName,
			// Phase 1追加: 言語とドメイン
			Language:    language,
			Domain:      domain,
		})

		prepared.processedFiles++
		prepared.totalChunks += len(chunkPayloads)

		idx.logger.Info("Prepared document",
			"path", doc.Path,
			"chunks", len(chunkPayloads),
			"contentType", contentType,
		)
	}

	return prepared, nil
}

// detectDeletedPaths は前回ハッシュに存在し現在欠落しているパスを列挙します
func detectDeletedPaths(previousFileHashes map[string]string, currentDocPaths map[string]bool) []string {
	if len(previousFileHashes) == 0 {
		return nil
	}

	// 前回存在し現在欠落しているパスのみ抽出
	var deleted []string
	for path := range previousFileHashes {
		if !currentDocPaths[path] {
			deleted = append(deleted, path)
		}
	}

	return deleted
}

// prepareChunks はチャンク化済みデータに対してEmbeddingとハッシュを付与します
func (idx *Indexer) prepareChunks(ctx context.Context, chunksWithMeta []*chunker.ChunkWithMetadata, filePath string) ([]*preparedChunk, error) {
	prepared := make([]*preparedChunk, 0, len(chunksWithMeta))
	embeddingTexts := make([]string, 0, len(chunksWithMeta))

	// Embeddingコンテキストを構築
	for _, cwm := range chunksWithMeta {
		// Embedding用の拡張コンテキストを構築
		embeddingContext := idx.contextBuilder.BuildContext(cwm.Chunk, cwm.Metadata, filePath)
		embeddingTexts = append(embeddingTexts, embeddingContext)

		// メタデータにEmbeddingContextを保存（後でDBに保存）
		if cwm.Metadata != nil {
			cwm.Metadata.EmbeddingContext = &embeddingContext
		}
	}

	// バッチでEmbeddingを生成（最大100件ずつ）
	// コンテキスト付きテキストでEmbeddingを生成
	batchSize := 100
	for i := 0; i < len(embeddingTexts); i += batchSize {
		end := i + batchSize
		if end > len(embeddingTexts) {
			end = len(embeddingTexts)
		}

		batch := embeddingTexts[i:end]
		embeddings, err := idx.embedder.BatchEmbed(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embeddings: %w", err)
		}

		for j, cwm := range chunksWithMeta[i:end] {
			chunk := cwm.Chunk
			// チャンクハッシュを計算し、Embedding と紐付けて保持
			chunkHash := fmt.Sprintf("%x", sha256.Sum256([]byte(chunk.Content)))
			prepared = append(prepared, &preparedChunk{
				StartLine: chunk.StartLine,
				EndLine:   chunk.EndLine,
				Content:   chunk.Content,
				Tokens:    chunk.Tokens,
				Hash:      chunkHash,
				Embedding: embeddings[j],
				Metadata:  cwm.Metadata, // メタデータを保持（EmbeddingContext含む）
			})
		}
	}

	return prepared, nil
}

// logMetrics は収集したメトリクスをログ出力します
func (idx *Indexer) logMetrics() {
	if idx.metrics.ASTParseAttempts > 0 {
		idx.logger.Info("AST parsing metrics",
			"attempts", idx.metrics.ASTParseAttempts,
			"successes", idx.metrics.ASTParseSuccesses,
			"failures", idx.metrics.ASTParseFailures,
			"successRate", fmt.Sprintf("%.2f%%", idx.metrics.ASTParseSuccessRate()*100),
			"failureRate", fmt.Sprintf("%.2f%%", idx.metrics.ASTParseFailureRate()*100),
		)
	}

	if idx.metrics.MetadataExtractAttempts > 0 {
		idx.logger.Info("Metadata extraction metrics",
			"attempts", idx.metrics.MetadataExtractAttempts,
			"successes", idx.metrics.MetadataExtractSuccesses,
			"failures", idx.metrics.MetadataExtractFailures,
			"successRate", fmt.Sprintf("%.2f%%", idx.metrics.MetadataExtractSuccessRate()*100),
		)
	}

	if idx.metrics.HighCommentRatioExcluded > 0 {
		idx.logger.Info("Chunk quality metrics",
			"highCommentRatioExcluded", idx.metrics.HighCommentRatioExcluded,
		)
	}

	if len(idx.metrics.CyclomaticComplexities) > 0 {
		idx.logger.Info("Cyclomatic complexity distribution",
			"count", len(idx.metrics.CyclomaticComplexities),
			"p50", idx.metrics.CyclomaticComplexityP50(),
			"p95", idx.metrics.CyclomaticComplexityP95(),
			"p99", idx.metrics.CyclomaticComplexityP99(),
		)
	}
}

// detectLanguage はファイルパスとコンテンツから言語を検出します
func (idx *Indexer) detectLanguage(path string, content string) *string {
	// go-enryを使用して言語を検出
	language := enry.GetLanguage(filepath.Base(path), []byte(content))
	if language == "" {
		return nil
	}
	return &language
}

// classifyDomain はファイルパスからドメインを分類します
func (idx *Indexer) classifyDomain(path string) *string {
	lowerPath := strings.ToLower(path)

	// テストファイル（優先度高い順にチェック）
	if strings.Contains(lowerPath, "_test.go") ||
		strings.Contains(lowerPath, "_test.") ||
		strings.Contains(lowerPath, "/test/") ||
		strings.Contains(lowerPath, "/tests/") ||
		strings.Contains(lowerPath, "/__tests__/") ||
		strings.Contains(lowerPath, "/spec/") ||
		strings.HasPrefix(lowerPath, "test/") ||
		strings.HasPrefix(lowerPath, "tests/") ||
		strings.HasPrefix(lowerPath, "spec/") {
		domain := "tests"
		return &domain
	}

	// 運用スクリプト（ドキュメントより前にチェック）
	if strings.Contains(lowerPath, "/scripts/") ||
		strings.Contains(lowerPath, "/ops/") ||
		strings.HasPrefix(lowerPath, "scripts/") ||
		strings.HasPrefix(lowerPath, "ops/") ||
		strings.HasSuffix(lowerPath, ".sh") ||
		strings.HasSuffix(lowerPath, ".bash") {
		domain := "ops"
		return &domain
	}

	// ドキュメント
	if strings.Contains(lowerPath, "/docs/") ||
		strings.Contains(lowerPath, "/doc/") ||
		strings.HasPrefix(lowerPath, "docs/") ||
		strings.HasPrefix(lowerPath, "doc/") ||
		strings.HasSuffix(lowerPath, ".md") ||
		strings.HasSuffix(lowerPath, ".markdown") ||
		strings.HasSuffix(lowerPath, ".rst") ||
		strings.HasSuffix(lowerPath, ".adoc") {
		domain := "architecture"
		return &domain
	}

	// インフラストラクチャ
	if strings.Contains(lowerPath, "dockerfile") ||
		strings.Contains(lowerPath, "docker-compose") ||
		strings.HasSuffix(lowerPath, ".yml") ||
		strings.HasSuffix(lowerPath, ".yaml") ||
		strings.Contains(lowerPath, "/terraform/") ||
		strings.Contains(lowerPath, "/ansible/") ||
		strings.Contains(lowerPath, "/k8s/") ||
		strings.Contains(lowerPath, "/kubernetes/") ||
		strings.HasPrefix(lowerPath, "terraform/") ||
		strings.HasPrefix(lowerPath, "ansible/") ||
		strings.HasPrefix(lowerPath, "k8s/") ||
		strings.HasPrefix(lowerPath, "kubernetes/") ||
		strings.HasSuffix(lowerPath, ".tf") {
		domain := "infra"
		return &domain
	}

	// デフォルトはcode
	domain := "code"
	return &domain
}

// persistPreparedChunks は事前計算したチャンク/EmbeddingをDBへ保存します
func (idx *Indexer) persistPreparedChunks(ctx context.Context, indexRepo *repository.IndexRepositoryRW, fileID, snapshotID uuid.UUID, file *preparedFile, preparedChunks []*preparedChunk) error {
	for ordinal, chunk := range preparedChunks {
		// chunk_keyを生成: {product_name}/{source_name}/{file_path}#L{start}-L{end}@{commit_hash}
		chunkKey := fmt.Sprintf("%s/%s/%s#L%d-L%d@%s",
			file.ProductName,
			file.SourceName,
			file.Path,
			chunk.StartLine,
			chunk.EndLine,
			file.CommitHash,
		)

		// デバッグログ: 最初のチャンクのみchunk_keyを出力
		if ordinal == 0 {
			idx.logger.Debug("Generated chunk_key",
				"chunk_key", chunkKey,
				"author", file.Author,
				"updated_at", file.UpdatedAt,
			)
		}

		// メタデータが存在しない場合は新規作成
		metadata := chunk.Metadata
		if metadata == nil {
			metadata = &repository.ChunkMetadata{}
		}

		// トレーサビリティ情報とchunk_keyを設定
		metadata.SourceSnapshotID = &snapshotID
		metadata.GitCommitHash = &file.CommitHash
		metadata.Author = &file.Author
		metadata.UpdatedAt = &file.UpdatedAt
		metadata.IsLatest = true
		metadata.ChunkKey = chunkKey

		// チャンク本体を保存（メタデータ付き）
		createdChunk, err := indexRepo.CreateChunk(
			ctx,
			fileID,
			ordinal,
			chunk.StartLine,
			chunk.EndLine,
			chunk.Content,
			chunk.Hash,
			chunk.Tokens,
			metadata,
		)
		if err != nil {
			return fmt.Errorf("failed to create chunk: %w", err)
		}

		// Embedding を同チャンクに紐付けて保存
		if err := indexRepo.CreateEmbedding(
			ctx,
			createdChunk.ID,
			chunk.Embedding,
			idx.embedder.GetModelName(),
		); err != nil {
			return fmt.Errorf("failed to create embedding: %w", err)
		}
	}

	return nil
}

// commitPreparedDocuments はロック取得後にスナップショット作成と永続化を完了させます
func (idx *Indexer) commitPreparedDocuments(ctx context.Context, params *commitPreparedDocumentParams) (*IndexResult, error) {
	// 書き込みトランザクション：ロック取得とDB更新
	return txprovider.Transact(ctx, idx.txProvider, func(adapters *txprovider.Adapter) (*IndexResult, error) {
		// 同一ソースの競合を避けるため advisory lock を取得
		advisoryLock, err := adapters.Locks.Acquire(ctx, params.lockID)
		if err != nil {
			return nil, err
		}
		defer advisoryLock.Release(ctx)

		// アドバイザリロック取得後、既存スナップショットをチェック
		existingSnapshot, err := adapters.Sources.GetSnapshotByVersion(ctx, params.source.ID, params.versionIdentifier)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("failed to check existing snapshot: %w", err)
		}

		// 既に存在する場合は早期リターン
		if existingSnapshot != nil {
			idx.logger.Info("Snapshot already exists, skipping indexing",
				"snapshotID", existingSnapshot.ID,
				"versionIdentifier", params.versionIdentifier,
			)

			duration := time.Since(params.startTime)
			return &IndexResult{
				SnapshotID:        existingSnapshot.ID.String(),
				VersionIdentifier: params.versionIdentifier,
				Duration:          duration,
			}, nil
		}

		// スナップショットを作成
		snapshot, err := adapters.Sources.CreateSnapshot(ctx, params.source.ID, params.versionIdentifier)
		if err != nil {
			return nil, fmt.Errorf("failed to create snapshot: %w", err)
		}

		// 前回スナップショットから削除対象を反映
		if len(params.deletedPaths) > 0 && params.previousSnapshot != nil {
			idx.logger.Info("Deleting documents", "count", len(params.deletedPaths))
			if err := adapters.Index.DeleteFilesByPaths(ctx, params.previousSnapshot.ID, params.deletedPaths); err != nil {
				return nil, fmt.Errorf("failed to delete files: %w", err)
			}
		}

		// 準備済みファイルを保存し、チャンク/Embedding を永続化
		for _, file := range params.preparedFiles {
			createdFile, err := adapters.Index.CreateFile(
				ctx,
				snapshot.ID,
				file.Path,
				file.Size,
				file.ContentType,
				file.ContentHash,
				file.Language, // Phase 1: 言語情報を渡す
				file.Domain,   // Phase 1: ドメイン情報を渡す
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s: %w", file.Path, err)
			}

			if err := idx.persistPreparedChunks(ctx, adapters.Index, createdFile.ID, snapshot.ID, file, file.Chunks); err != nil {
				return nil, fmt.Errorf("failed to persist chunks for file %s: %w", file.Path, err)
			}
		}

		// スナップショットをインデックス完了へ更新
		if err := adapters.Sources.MarkSnapshotIndexed(ctx, snapshot.ID); err != nil {
			return nil, fmt.Errorf("failed to mark snapshot as indexed: %w", err)
		}

		// 最終結果を構築
		duration := time.Since(params.startTime)
		return &IndexResult{
			SnapshotID:        snapshot.ID.String(),
			VersionIdentifier: params.versionIdentifier,
			ProcessedFiles:    params.processedFiles,
			TotalChunks:       params.totalChunks,
			Duration:          duration,
		}, nil
	})
}
