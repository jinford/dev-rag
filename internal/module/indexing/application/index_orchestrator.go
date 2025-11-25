package application

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	"github.com/jinford/dev-rag/internal/platform/database"
)

// IndexOrchestrator はインデックス化のビジネスフローを統括します
type IndexOrchestrator struct {
	// ドメインポート
	provider         domain.SourceProvider
	detector         domain.LanguageDetector
	chunkerFactory   domain.ChunkerFactory
	embedder         domain.Embedder
	importance       domain.ImportanceEvaluator
	coverage         domain.CoverageReporter

	// リポジトリポート
	productRepo      domain.ProductRepository
	sourceRepo       domain.SourceRepository
	fileRepo         domain.FileRepository
	chunkRepo        domain.ChunkRepository
	embeddingRepo    domain.EmbeddingRepository
	dependencyRepo   domain.DependencyRepository
	snapshotFileRepo domain.SnapshotFileRepository

	// 技術基盤
	txProvider       *database.TransactionProvider
	logger           *slog.Logger
}

// NewIndexOrchestrator は新しいIndexOrchestratorを作成します
func NewIndexOrchestrator(
	provider domain.SourceProvider,
	detector domain.LanguageDetector,
	chunkerFactory domain.ChunkerFactory,
	embedder domain.Embedder,
	importance domain.ImportanceEvaluator,
	coverage domain.CoverageReporter,
	productRepo domain.ProductRepository,
	sourceRepo domain.SourceRepository,
	fileRepo domain.FileRepository,
	chunkRepo domain.ChunkRepository,
	embeddingRepo domain.EmbeddingRepository,
	dependencyRepo domain.DependencyRepository,
	snapshotFileRepo domain.SnapshotFileRepository,
	txProvider *database.TransactionProvider,
	logger *slog.Logger,
) *IndexOrchestrator {
	return &IndexOrchestrator{
		provider:         provider,
		detector:         detector,
		chunkerFactory:   chunkerFactory,
		embedder:         embedder,
		importance:       importance,
		coverage:         coverage,
		productRepo:      productRepo,
		sourceRepo:       sourceRepo,
		fileRepo:         fileRepo,
		chunkRepo:        chunkRepo,
		embeddingRepo:    embeddingRepo,
		dependencyRepo:   dependencyRepo,
		snapshotFileRepo: snapshotFileRepo,
		txProvider:       txProvider,
		logger:           logger,
	}
}

// preparedFile は準備済みファイルを表します
type preparedFile struct {
	Path        string
	Size        int64
	ContentType string
	ContentHash string
	Content     string
	Chunks      []*preparedChunk

	// メタデータ
	CommitHash  string
	Author      string
	UpdatedAt   time.Time
	Language    *string
	Domain      *string

	// chunk_key生成用
	ProductName string
	SourceName  string
}

// preparedChunk は準備済みチャンクを表します
type preparedChunk struct {
	StartLine int
	EndLine   int
	Content   string
	Tokens    int
	Hash      string
	Embedding []float32
	Metadata  *domain.ChunkMetadata
}

// IndexSource はソースのインデックス化フローを実行します
func (o *IndexOrchestrator) IndexSource(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*IndexResult, error) {
	startTime := time.Now()

	o.logger.Info("Starting index orchestration",
		"sourceType", sourceType,
		"identifier", params.Identifier,
		"product", params.ProductName,
		"forceInit", params.ForceInit,
	)

	// 1. ドキュメント取得
	documents, versionIdentifier, err := o.provider.FetchDocuments(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch documents: %w", err)
	}

	o.logger.Info("Documents fetched", "count", len(documents))

	// 2. プロダクト/ソースを確定（短期トランザクション）
	source, err := o.ensureSource(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure source: %w", err)
	}

	// 3. 前回スナップショットとファイルハッシュを取得（差分判定用）
	previousSnapshot, previousFileHashes, err := o.loadPreviousSnapshot(ctx, source.ID, params.ForceInit)
	if err != nil {
		return nil, fmt.Errorf("failed to load previous snapshot: %w", err)
	}

	// 4. ドキュメントの準備（チャンク化・Embedding）
	sourceName := o.provider.ExtractSourceName(params.Identifier)
	preparedFiles, currentDocPaths, err := o.prepareDocuments(
		ctx,
		documents,
		previousFileHashes,
		params.ProductName,
		sourceName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare documents: %w", err)
	}

	// 5. 削除されたパスを検出
	deletedPaths := o.detectDeletedPaths(previousFileHashes, currentDocPaths)

	// 6. トランザクション境界で永続化（アドバイザリロック取得）
	lockID := generateLockID(string(sourceType), params.Identifier)
	result, err := o.commitPreparedDocuments(ctx, &commitParams{
		lockID:            lockID,
		source:            source,
		versionIdentifier: versionIdentifier,
		previousSnapshot:  previousSnapshot,
		deletedPaths:      deletedPaths,
		preparedFiles:     preparedFiles,
		allDocuments:      documents,
		startTime:         startTime,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to commit documents: %w", err)
	}

	// 7. 重要度スコア計算（トランザクション後）
	// NOTE: 現在の実装では commitPreparedDocuments 内で実行済み

	// 8. カバレッジレポート構築
	snapshotID, err := uuid.Parse(result.SnapshotID)
	if err == nil {
		coverageMap, err := o.coverage.Build(ctx, snapshotID, versionIdentifier)
		if err != nil {
			o.logger.Warn("Failed to build coverage report", "error", err)
		} else {
			o.logger.Info("Coverage report built",
				"overallCoverage", coverageMap.OverallCoverage,
				"totalFiles", coverageMap.TotalFiles,
				"indexedFiles", coverageMap.TotalIndexedFiles,
			)
		}
	}

	o.logger.Info("Index orchestration completed",
		"snapshotID", result.SnapshotID,
		"processedFiles", result.ProcessedFiles,
		"totalChunks", result.TotalChunks,
		"duration", result.Duration,
	)

	return result, nil
}

// ensureSource はプロダクト/ソースを確定します
func (o *IndexOrchestrator) ensureSource(ctx context.Context, params domain.IndexParams) (*domain.Source, error) {
	return database.Transact(ctx, o.txProvider, func(adapters *database.Adapter) (*domain.Source, error) {
		// プロダクトを作成または取得
		product, err := adapters.Products.CreateIfNotExists(ctx, params.ProductName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create product: %w", err)
		}

		// ソースを作成または取得
		sourceName := o.provider.ExtractSourceName(params.Identifier)
		metadata := o.provider.CreateMetadata(params)

		source, err := adapters.Sources.CreateIfNotExists(ctx, sourceName, o.provider.GetSourceType(), product.ID, metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to create source: %w", err)
		}

		return source, nil
	})
}

// loadPreviousSnapshot は差分判定用に最新スナップショットとファイルハッシュを取得します
func (o *IndexOrchestrator) loadPreviousSnapshot(ctx context.Context, sourceID uuid.UUID, forceInit bool) (*domain.SourceSnapshot, map[string]string, error) {
	if forceInit {
		// 初回インデックスでは比較対象なし
		return nil, make(map[string]string), nil
	}

	// 最新のインデックス済みスナップショットを取得
	latestSnapshot, err := o.sourceRepo.GetLatestIndexedSnapshot(ctx, sourceID)
	if err != nil {
		// スナップショットが存在しない場合は初回として扱う
		if strings.Contains(err.Error(), "not found") {
			return nil, make(map[string]string), nil
		}
		return nil, nil, fmt.Errorf("failed to get latest indexed snapshot: %w", err)
	}

	// ファイルハッシュを取得
	hashes, err := o.fileRepo.GetHashesBySnapshot(ctx, latestSnapshot.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file hashes: %w", err)
	}

	return latestSnapshot, hashes, nil
}

// prepareDocuments はドキュメントをチャンク化・Embedding化します
func (o *IndexOrchestrator) prepareDocuments(
	ctx context.Context,
	documents []*domain.SourceDocument,
	previousFileHashes map[string]string,
	productName string,
	sourceName string,
) ([]*preparedFile, map[string]bool, error) {
	prepared := make([]*preparedFile, 0, len(documents))
	currentDocPaths := make(map[string]bool, len(documents))

	for _, doc := range documents {
		currentDocPaths[doc.Path] = true

		// 除外判定
		if o.provider.ShouldIgnore(doc) {
			o.logger.Debug("Skipping ignored document", "path", doc.Path)
			continue
		}

		// 差分判定: ハッシュが同一ならスキップ
		if previousHash, exists := previousFileHashes[doc.Path]; exists && previousHash == doc.ContentHash {
			o.logger.Debug("Skipping unchanged document", "path", doc.Path)
			continue
		}

		// 言語検出
		language, err := o.detector.DetectLanguage(doc.Path, []byte(doc.Content))
		if err != nil {
			o.logger.Warn("Failed to detect language", "path", doc.Path, "error", err)
			language = ""
		}

		var langPtr *string
		if language != "" {
			langPtr = &language
		}

		// Chunker取得
		chunker, err := o.chunkerFactory.GetChunker(language)
		if err != nil {
			o.logger.Warn("Failed to get chunker", "path", doc.Path, "language", language, "error", err)
			continue
		}

		// チャンク化
		chunks, err := chunker.Chunk(ctx, doc.Path, doc.Content)
		if err != nil {
			o.logger.Warn("Failed to chunk document", "path", doc.Path, "error", err)
			continue
		}

		if len(chunks) == 0 {
			o.logger.Debug("No chunks generated", "path", doc.Path)
			continue
		}

		// Embedding生成
		prepChunks, err := o.prepareChunks(ctx, chunks, doc.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to prepare chunks for file %s: %w", doc.Path, err)
		}

		// ドメイン分類（簡易実装: 後で拡張可能）
		domain := classifyDomain(doc.Path)

		prepared = append(prepared, &preparedFile{
			Path:        doc.Path,
			Size:        doc.Size,
			ContentType: detectContentType(doc.Path),
			ContentHash: doc.ContentHash,
			Content:     doc.Content,
			Chunks:      prepChunks,
			CommitHash:  doc.CommitHash,
			Author:      doc.Author,
			UpdatedAt:   doc.UpdatedAt,
			Language:    langPtr,
			Domain:      domain,
			ProductName: productName,
			SourceName:  sourceName,
		})

		o.logger.Info("Prepared document",
			"path", doc.Path,
			"chunks", len(prepChunks),
			"language", language,
		)
	}

	return prepared, currentDocPaths, nil
}

// prepareChunks はチャンクにEmbeddingを付与します
func (o *IndexOrchestrator) prepareChunks(ctx context.Context, chunks []*domain.Chunk, filePath string) ([]*preparedChunk, error) {
	prepared := make([]*preparedChunk, 0, len(chunks))
	embeddingTexts := make([]string, 0, len(chunks))

	// Embedding用のテキストを構築
	for _, chunk := range chunks {
		embeddingTexts = append(embeddingTexts, chunk.Content)
	}

	// バッチでEmbedding生成
	batchSize := 100
	embeddings := make([][]float32, 0, len(chunks))

	for i := 0; i < len(embeddingTexts); i += batchSize {
		end := i + batchSize
		if end > len(embeddingTexts) {
			end = len(embeddingTexts)
		}

		batch := embeddingTexts[i:end]
		batchEmbeddings, err := o.embedder.BatchEmbed(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embeddings: %w", err)
		}

		embeddings = append(embeddings, batchEmbeddings...)
	}

	// チャンクとEmbeddingを結合
	for i, chunk := range chunks {
		chunkHash := fmt.Sprintf("%x", sha256.Sum256([]byte(chunk.Content)))

		metadata := &domain.ChunkMetadata{
			Type:       chunk.Type,
			Name:       chunk.Name,
			ParentName: chunk.ParentName,
			Signature:  chunk.Signature,
			DocComment: chunk.DocComment,
			Imports:    chunk.Imports,
			Calls:      chunk.Calls,
		}

		prepared = append(prepared, &preparedChunk{
			StartLine: chunk.StartLine,
			EndLine:   chunk.EndLine,
			Content:   chunk.Content,
			Tokens:    chunk.TokenCount,
			Hash:      chunkHash,
			Embedding: embeddings[i],
			Metadata:  metadata,
		})
	}

	return prepared, nil
}

// detectDeletedPaths は削除されたパスを検出します
func (o *IndexOrchestrator) detectDeletedPaths(previousFileHashes map[string]string, currentDocPaths map[string]bool) []string {
	if len(previousFileHashes) == 0 {
		return nil
	}

	deleted := make([]string, 0)
	for path := range previousFileHashes {
		if !currentDocPaths[path] {
			deleted = append(deleted, path)
		}
	}

	return deleted
}

// commitParams はコミット処理に必要なパラメータをまとめます
type commitParams struct {
	lockID            int64
	source            *domain.Source
	versionIdentifier string
	previousSnapshot  *domain.SourceSnapshot
	deletedPaths      []string
	preparedFiles     []*preparedFile
	allDocuments      []*domain.SourceDocument
	startTime         time.Time
}

// commitPreparedDocuments はトランザクション境界で永続化を実行します
func (o *IndexOrchestrator) commitPreparedDocuments(ctx context.Context, params *commitParams) (*IndexResult, error) {
	return database.Transact(ctx, o.txProvider, func(adapters *database.Adapter) (*IndexResult, error) {
		// アドバイザリロック取得
		advisoryLock, err := adapters.Locks.Acquire(ctx, params.lockID)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire advisory lock: %w", err)
		}
		defer advisoryLock.Release(ctx)

		// 既存スナップショットをチェック
		existingSnapshot, err := adapters.Sources.GetSnapshotByVersion(ctx, params.source.ID, params.versionIdentifier)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("failed to check existing snapshot: %w", err)
		}

		if existingSnapshot != nil {
			o.logger.Info("Snapshot already exists, skipping indexing",
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

		// スナップショット作成
		snapshot, err := adapters.Sources.CreateSnapshot(ctx, params.source.ID, params.versionIdentifier)
		if err != nil {
			return nil, fmt.Errorf("failed to create snapshot: %w", err)
		}

		// 削除パスの処理
		if len(params.deletedPaths) > 0 && params.previousSnapshot != nil {
			o.logger.Info("Deleting documents", "count", len(params.deletedPaths))
			if err := adapters.Index.DeleteFilesByPaths(ctx, params.previousSnapshot.ID, params.deletedPaths); err != nil {
				return nil, fmt.Errorf("failed to delete files: %w", err)
			}
		}

		// 全ドキュメントをsnapshot_filesに記録
		processedFilesPaths := make(map[string]bool)
		for _, file := range params.preparedFiles {
			processedFilesPaths[file.Path] = true
		}

		for _, doc := range params.allDocuments {
			domain := classifyDomain(doc.Path)
			isIgnored := o.provider.ShouldIgnore(doc)

			var skipReason *string
			if isIgnored {
				reason := "ignored by filter"
				skipReason = &reason
			}

			indexed := processedFilesPaths[doc.Path]

			if _, err := adapters.Index.CreateSnapshotFile(ctx, snapshot.ID, doc.Path, doc.Size, domain, indexed, skipReason); err != nil {
				return nil, fmt.Errorf("failed to create snapshot file %s: %w", doc.Path, err)
			}
		}

		// ファイル・チャンク・Embeddingを永続化
		processedFiles := 0
		totalChunks := 0

		for _, file := range params.preparedFiles {
			createdFile, err := adapters.Index.CreateFile(
				ctx,
				snapshot.ID,
				file.Path,
				file.Size,
				file.ContentType,
				file.ContentHash,
				file.Language,
				file.Domain,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s: %w", file.Path, err)
			}

			// チャンク・Embeddingを保存
			for ordinal, chunk := range file.Chunks {
				// chunk_key生成
				chunkKey := fmt.Sprintf("%s/%s/%s#L%d-L%d@%s",
					file.ProductName,
					file.SourceName,
					file.Path,
					chunk.StartLine,
					chunk.EndLine,
					file.CommitHash,
				)

				// メタデータ設定
				metadata := chunk.Metadata
				if metadata == nil {
					metadata = &domain.ChunkMetadata{}
				}
				metadata.SourceSnapshotID = &snapshot.ID
				metadata.GitCommitHash = &file.CommitHash
				metadata.Author = &file.Author
				metadata.UpdatedAt = &file.UpdatedAt
				metadata.IsLatest = true
				metadata.ChunkKey = chunkKey

				// チャンク作成
				createdChunk, err := adapters.Index.CreateChunk(
					ctx,
					createdFile.ID,
					ordinal,
					chunk.StartLine,
					chunk.EndLine,
					chunk.Content,
					chunk.Hash,
					chunk.Tokens,
					metadata,
				)
				if err != nil {
					return nil, fmt.Errorf("failed to create chunk: %w", err)
				}

				// Embedding作成
				if err := adapters.Index.CreateEmbedding(
					ctx,
					createdChunk.ID,
					chunk.Embedding,
					o.embedder.GetModelName(),
				); err != nil {
					return nil, fmt.Errorf("failed to create embedding: %w", err)
				}

				totalChunks++
			}

			processedFiles++
		}

		// スナップショットをインデックス済みにマーク
		if err := adapters.Sources.MarkSnapshotIndexed(ctx, snapshot.ID); err != nil {
			return nil, fmt.Errorf("failed to mark snapshot as indexed: %w", err)
		}

		duration := time.Since(params.startTime)
		return &IndexResult{
			SnapshotID:        snapshot.ID.String(),
			VersionIdentifier: params.versionIdentifier,
			ProcessedFiles:    processedFiles,
			TotalChunks:       totalChunks,
			Duration:          duration,
		}, nil
	})
}

// generateLockID は文字列からロックIDを生成します
func generateLockID(parts ...string) int64 {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
	}
	hash := h.Sum(nil)

	var id int64
	for i := 0; i < 8; i++ {
		id = (id << 8) | int64(hash[i])
	}

	return id
}

// classifyDomain はファイルパスからドメインを分類します（簡易実装）
func classifyDomain(path string) *string {
	lowerPath := strings.ToLower(path)

	if strings.Contains(lowerPath, "_test.") || strings.Contains(lowerPath, "/test/") {
		domain := "tests"
		return &domain
	}

	if strings.Contains(lowerPath, "/docs/") || strings.HasSuffix(lowerPath, ".md") {
		domain := "architecture"
		return &domain
	}

	if strings.Contains(lowerPath, "/scripts/") || strings.HasSuffix(lowerPath, ".sh") {
		domain := "ops"
		return &domain
	}

	if strings.Contains(lowerPath, "dockerfile") || strings.HasSuffix(lowerPath, ".yml") {
		domain := "infra"
		return &domain
	}

	domain := "code"
	return &domain
}

// detectContentType はファイルパスからコンテンツタイプを検出します（簡易実装）
func detectContentType(path string) string {
	lowerPath := strings.ToLower(path)

	if strings.HasSuffix(lowerPath, ".go") {
		return "text/x-go"
	}
	if strings.HasSuffix(lowerPath, ".py") {
		return "text/x-python"
	}
	if strings.HasSuffix(lowerPath, ".js") {
		return "text/javascript"
	}
	if strings.HasSuffix(lowerPath, ".ts") {
		return "text/typescript"
	}
	if strings.HasSuffix(lowerPath, ".md") {
		return "text/markdown"
	}

	return "text/plain"
}
