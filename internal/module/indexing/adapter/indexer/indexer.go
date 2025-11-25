package indexer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-enry/go-enry/v2"
	"github.com/google/uuid"

	gitprovider "github.com/jinford/dev-rag/internal/module/indexing/adapter/git"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/chunker"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/coverage"
	embedpkg "github.com/jinford/dev-rag/internal/module/indexing/adapter/embedder"
	indexingpg "github.com/jinford/dev-rag/internal/module/indexing/adapter/pg"
	"github.com/jinford/dev-rag/internal/module/indexing/adapter/prompts"
	indexingapp "github.com/jinford/dev-rag/internal/module/indexing/application"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	domaindep "github.com/jinford/dev-rag/internal/module/indexing/domain/dependency"
	"github.com/jinford/dev-rag/internal/platform/database"
)

// Indexer はソースのインデックス化を管理します
type Indexer struct {
	sourceReadRepo      *indexingpg.SourceRepository
	indexReadRepo       *indexingpg.IndexRepositoryR
	txProvider          *database.TransactionProvider
	srcProviders        map[domain.SourceType]domain.SourceProvider
	chunker             *chunker.Chunker
	embedder            domain.Embedder
	contextBuilder      *embedpkg.ContextBuilder
	detector            domain.Detector
	gitClient           *gitprovider.GitClient                // Git履歴取得用
	domainClassifier    *prompts.DomainClassifier            // LLMドメイン分類器
	useLLMClassifier    bool                                 // LLM分類を使用するかのフラグ
	fileSummaryService  *indexingapp.FileSummaryService      // ファイル要約サービス（必須）
	logger              *slog.Logger
	metrics             *IndexMetrics // メトリクス収集
}

// NewIndexer は新しいIndexerを作成します
func NewIndexer(
	sourceRepo *indexingpg.SourceRepository,
	indexRepo *indexingpg.IndexRepositoryR,
	txProvider *database.TransactionProvider,
	chunker *chunker.Chunker,
	embedder domain.Embedder,
	detector domain.Detector,
	gitClient *gitprovider.GitClient,
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
		srcProviders:   make(map[domain.SourceType]domain.SourceProvider),
		chunker:        chunker,
		embedder:       embedder,
		contextBuilder: contextBuilder,
		detector:       detector,
		gitClient:      gitClient,
		logger:         logger,
		metrics:        NewIndexMetrics(), // メトリクス初期化
	}, nil
}

// RegisterProvider はソースプロバイダーを登録します
func (idx *Indexer) RegisterProvider(srcProvider domain.SourceProvider) {
	idx.srcProviders[srcProvider.GetSourceType()] = srcProvider
}

// SetDomainClassifier はLLMドメイン分類器を設定します
// LLMドメイン分類の統合
func (idx *Indexer) SetDomainClassifier(classifier *prompts.DomainClassifier) {
	idx.domainClassifier = classifier
	idx.useLLMClassifier = true
}

// SetFileSummaryService はファイル要約サービスを設定します
// ファイル要約をfile_summariesテーブルに保存する（必須設定）
func (idx *Indexer) SetFileSummaryService(service *indexingapp.FileSummaryService) {
	idx.fileSummaryService = service
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
func (idx *Indexer) IndexSource(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*IndexResult, error) {
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
	lockID := database.GenerateLockID(string(prov.GetSourceType()), params.Identifier)

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
		allDocuments:      preparedDocs.allDocuments,  // 全ドキュメントを渡す
		sourceProvider:    prov,                       // プロバイダーを渡す
	})
	if err != nil {
		return nil, err
	}

	// トランザクション完了後、ファイル要約を同期的に生成（Wiki要約生成の前提条件）
	if err := idx.generateFileSummaries(ctx, preparedDocs.files); err != nil {
		// ファイル要約生成に失敗してもインデックス化は成功とみなす（警告のみ）
		idx.logger.Warn("Failed to generate file summaries", "error", err)
	}

	// カバレッジアラートを生成・表示
	idx.generateAndDisplayCoverageAlerts(ctx, result.SnapshotID, versionIdentifier)

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
	allDocuments    []*domain.SourceDocument // 全ドキュメント（カバレッジ計算用）
}

type preparedFile struct {
	Path        string
	Size        int64
	ContentType string
	ContentHash string
	Content     string // ファイルコンテンツ（ファイル要約生成用）
	Chunks      []*preparedChunk

	// コミットメタデータ
	CommitHash string
	Author     string
	UpdatedAt  time.Time

	// chunk_key生成用
	ProductName string
	SourceName  string

	// 言語とドメイン
	Language *string
	Domain   *string

	// 生成されたFileID（要約生成用）
	FileID uuid.UUID
}

type preparedChunk struct {
	StartLine int
	EndLine   int
	Content   string
	Tokens    int
	Hash      string
	Embedding []float32
	Metadata  *domain.ChunkMetadata
}

// commitPreparedDocumentParams は書き込み処理に必要な情報をまとめます
type commitPreparedDocumentParams struct {
	lockID            int64
	source            *domain.Source
	versionIdentifier string
	previousSnapshot  *domain.SourceSnapshot
	deletedPaths      []string
	preparedFiles     []*preparedFile
	processedFiles    int
	totalChunks       int
	startTime         time.Time
	allDocuments      []*domain.SourceDocument // 全ドキュメント（カバレッジ計算用）
	sourceProvider    domain.SourceProvider    // プロバイダー（除外判定用）
}

// loadPreviousSnapshot は差分判定用に最新インデックス済みスナップショットとファイルハッシュを読み出します
func (idx *Indexer) loadPreviousSnapshot(ctx context.Context, source *domain.Source, forceInit bool) (*domain.SourceSnapshot, map[string]string, error) {
	if forceInit {
		// 初回インデックスでは比較対象を空集合にする
		return nil, make(map[string]string), nil
	}

	// 最新のインデックス済みスナップショットを取得
	latestSnapshot, err := idx.sourceReadRepo.GetLatestIndexedSnapshot(ctx, source.ID)
	if err != nil {
		// スナップショットが存在しない場合は初回インデックスとして扱う
		if strings.Contains(err.Error(), "not found") {
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
func (idx *Indexer) ensureSource(ctx context.Context, prov domain.SourceProvider, params domain.IndexParams) (*domain.Source, error) {
	return database.Transact(ctx, idx.txProvider, func(adapters *database.Adapter) (*domain.Source, error) {
		// プロダクトを存在しなければ作成
		product, err := adapters.Products.CreateIfNotExists(ctx, params.ProductName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create product: %w", err)
		}
		productID := product.ID

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
func (idx *Indexer) prepareDocuments(ctx context.Context, prov domain.SourceProvider, documents []*domain.SourceDocument, previousFileHashes map[string]string, productName, sourceName string) (*preparedDocumentsResult, error) {
	prepared := &preparedDocumentsResult{
		files:           make([]*preparedFile, 0, len(documents)),
		currentDocPaths: make(map[string]bool, len(documents)),
		allDocuments:    documents, // 全ドキュメントを保持（カバレッジ計算用）
	}

	for _, doc := range documents {
		// 取得できたドキュメントのパスを記録
		prepared.currentDocPaths[doc.Path] = true

		// 除外設定に合致すればスキップ（カバレッジ計算では記録）
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
			Content:     doc.Content, // ファイルコンテンツを保持（ファイル要約生成用）
			Chunks:      chunkPayloads,
			// コミットメタデータを保持
			CommitHash:  doc.CommitHash,
			Author:      doc.Author,
			UpdatedAt:   doc.UpdatedAt,
			// chunk_key生成用の情報を保持
			ProductName: productName,
			SourceName:  sourceName,
			// 言語とドメイン
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
// LLM分類とルールベース分類の統合
func (idx *Indexer) classifyDomain(path string) *string {
	// ルールベースドメイン分類を実行
	ruleBasedDomain := idx.classifyDomainRuleBased(path)

	// LLM分類が無効な場合はルールベース結果をそのまま返す
	if !idx.useLLMClassifier || idx.domainClassifier == nil {
		return ruleBasedDomain
	}

	// LLM分類を実行（ここでは同期的に実行）
	// 実際の本番環境では非同期・バッチ処理が推奨される
	domain := idx.classifyDomainWithLLM(path, ruleBasedDomain)
	return domain
}

// classifyDomainRuleBased はルールベースでファイルパスからドメインを分類します
func (idx *Indexer) classifyDomainRuleBased(path string) *string {
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

// classifyDomainWithLLM はLLMを使用してドメインを分類します
// LLMドメイン分類の実装
func (idx *Indexer) classifyDomainWithLLM(path string, ruleBasedDomain *string) *string {
	// TODO: 実際のファイルコンテンツを読み込む必要がある場合は、
	// prepareDocumentsの段階でファイル情報を渡す設計に変更する必要がある
	// ここでは簡易実装としてルールベース結果をフォールバックとして使用

	// 信頼度の閾値
	const confidenceThreshold = 0.5

	// サンプル行を抽出（ここでは空文字列、実際の実装では実ファイル読み込みが必要）
	sampleLines := ""

	// ディレクトリヒントを生成
	directoryHint := prompts.CreateDirectoryHintFromRuleBased(path, ruleBasedDomain)

	// 言語検出
	detectedLanguage := ""
	if lang := idx.detectLanguage(path, ""); lang != nil {
		detectedLanguage = *lang
	}

	// LLM分類リクエストを構築
	req := prompts.DomainClassificationRequest{
		NodePath:         path,
		NodeType:         "file",
		DetectedLanguage: detectedLanguage,
		SampleLines:      sampleLines,
		DirectoryHints:   directoryHint,
	}

	// LLM分類を実行
	ctx := context.Background()
	resp, err := idx.domainClassifier.Classify(ctx, req)
	if err != nil {
		idx.logger.Warn("LLM domain classification failed, falling back to rule-based",
			"path", path,
			"error", err)
		return ruleBasedDomain
	}

	// 信頼度が低い場合はルールベース結果にフォールバック
	if resp.Confidence < confidenceThreshold {
		idx.logger.Debug("LLM classification confidence too low, using rule-based result",
			"path", path,
			"llm_domain", resp.Domain,
			"confidence", resp.Confidence,
			"rule_based_domain", *ruleBasedDomain)
		return ruleBasedDomain
	}

	// LLM分類結果を使用
	idx.logger.Debug("Using LLM domain classification",
		"path", path,
		"domain", resp.Domain,
		"confidence", resp.Confidence,
		"rationale", resp.Rationale)

	return &resp.Domain
}

// persistPreparedChunks は事前計算したチャンク/EmbeddingをDBへ保存します
func (idx *Indexer) persistPreparedChunks(ctx context.Context, indexRepo *indexingpg.IndexRepositoryRW, fileID, snapshotID uuid.UUID, file *preparedFile, preparedChunks []*preparedChunk) error {
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
			metadata = &domain.ChunkMetadata{}
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
	return database.Transact(ctx, idx.txProvider, func(adapters *database.Adapter) (*IndexResult, error) {
		// 同一ソースの競合を避けるため advisory lock を取得
		advisoryLock, err := adapters.Locks.Acquire(ctx, params.lockID)
		if err != nil {
			return nil, err
		}
		defer advisoryLock.Release(ctx)

		// アドバイザリロック取得後、既存スナップショットをチェック
		existingSnapshot, err := adapters.Sources.GetSnapshotByVersion(ctx, params.source.ID, params.versionIdentifier)
		if err != nil && !strings.Contains(err.Error(), "not found") {
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

		// 全ドキュメントをsnapshot_filesに記録（カバレッジ計算用）
		processedFilesPaths := make(map[string]bool)
		for _, file := range params.preparedFiles {
			processedFilesPaths[file.Path] = true
		}

		for _, doc := range params.allDocuments {
			// ドメイン分類を実行
			domain := idx.classifyDomain(doc.Path)

			// 除外判定
			isIgnored := params.sourceProvider.ShouldIgnore(doc)
			var skipReason *string
			if isIgnored {
				reason := "ignored by filter"
				skipReason = &reason
			}

			// インデックス済みかどうか判定
			indexed := processedFilesPaths[doc.Path]

			// snapshot_filesに記録
			if _, err := adapters.Index.CreateSnapshotFile(ctx, snapshot.ID, doc.Path, doc.Size, domain, indexed, skipReason); err != nil {
				return nil, fmt.Errorf("failed to create snapshot file %s: %w", doc.Path, err)
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
				file.Language, // 言語情報を渡す
				file.Domain,   // ドメイン情報を渡す
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s: %w", file.Path, err)
			}

			if err := idx.persistPreparedChunks(ctx, adapters.Index, createdFile.ID, snapshot.ID, file, file.Chunks); err != nil {
				return nil, fmt.Errorf("failed to persist chunks for file %s: %w", file.Path, err)
			}

			// FileIDを記録（トランザクション後にファイル要約を生成するため）
			file.FileID = createdFile.ID
		}

		// スナップショットをインデックス完了へ更新
		if err := adapters.Sources.MarkSnapshotIndexed(ctx, snapshot.ID); err != nil {
			return nil, fmt.Errorf("failed to mark snapshot as indexed: %w", err)
		}

		// 重要度スコアを計算・保存
		if err := idx.calculateAndSaveImportanceScores(ctx, adapters, snapshot.ID, params); err != nil {
			// エラーが発生してもログに記録するだけで処理は続行
			idx.logger.Warn("Failed to calculate importance scores", "error", err)
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

// calculateAndSaveImportanceScores は重要度スコアを計算してDBに保存します
func (idx *Indexer) calculateAndSaveImportanceScores(ctx context.Context, adapters *database.Adapter, snapshotID uuid.UUID, params *commitPreparedDocumentParams) error {
	// Gitプロバイダーでない場合はスキップ
	if params.sourceProvider.GetSourceType() != domain.SourceTypeGit {
		idx.logger.Debug("Skipping importance score calculation for non-Git source")
		return nil
	}

	// GitClientが設定されていない場合はスキップ
	if idx.gitClient == nil {
		idx.logger.Debug("GitClient is not configured, skipping importance score calculation")
		return nil
	}

	// ソースのメタデータからリポジトリパスを取得
	sourceMetadata := params.source.Metadata
	if sourceMetadata == nil {
		idx.logger.Debug("Source metadata is nil, skipping importance score calculation")
		return nil
	}

	// メタデータからローカルパスを取得（Git providerが設定するlocalPath）
	localPathInterface, ok := sourceMetadata["localPath"]
	if !ok {
		idx.logger.Debug("localPath not found in source metadata, skipping importance score calculation")
		return nil
	}

	repoPath, ok := localPathInterface.(string)
	if !ok || repoPath == "" {
		idx.logger.Debug("localPath is not a valid string, skipping importance score calculation")
		return nil
	}

	// 1. スナップショット配下の全チャンクを取得
	files, err := adapters.Index.ListFilesBySnapshot(ctx, snapshotID)
	if err != nil {
		return fmt.Errorf("failed to list files by snapshot: %w", err)
	}

	if len(files) == 0 {
		idx.logger.Debug("No files found for snapshot, skipping importance score calculation")
		return nil
	}

	// 2. 依存グラフを構築
	graph := domaindep.NewGraph()

	// チャンクIDからファイルパスへのマッピング
	chunkIDToFilePath := make(map[uuid.UUID]string)

	for _, file := range files {
		chunks, err := adapters.Index.ListChunksByFile(ctx, file.ID)
		if err != nil {
			idx.logger.Warn("Failed to list chunks for file", "fileID", file.ID, "error", err)
			continue
		}

		for _, chunk := range chunks {
			// グラフにノードを追加
			node := &domaindep.Node{
				ChunkID:  chunk.ID,
				Name:     stringPtrOrEmpty(chunk.Name),
				Type:     stringPtrOrEmpty(chunk.Type),
				FilePath: file.Path,
			}
			graph.AddNode(node)
			chunkIDToFilePath[chunk.ID] = file.Path

			// 依存関係（エッジ）を追加
			// 関数呼び出しからエッジを生成
			for _, call := range chunk.Calls {
				// 呼び出し先のチャンクを探す（名前ベースで簡易的にマッチング）
				targetChunkID := idx.findChunkIDByName(chunks, call)
				if targetChunkID != nil {
					edge := &domaindep.Edge{
						From:         chunk.ID,
						To:           *targetChunkID,
						RelationType: domaindep.RelationTypeCalls,
						Weight:       1,
					}
					if err := graph.AddEdge(edge); err != nil {
						idx.logger.Debug("Failed to add edge", "from", chunk.ID, "to", targetChunkID, "error", err)
					}
				}
			}
		}
	}

	// 3. Git履歴から編集頻度を取得（過去90日間）
	since := time.Now().AddDate(0, 0, -90)
	editFreqs, err := idx.gitClient.GetFileEditFrequencies(ctx, repoPath, params.versionIdentifier, since)
	if err != nil {
		idx.logger.Warn("Failed to get file edit frequencies", "error", err)
		// 編集頻度が取得できなくても続行（空のマップを使用）
		editFreqs = make(map[string]*gitprovider.FileEditFrequency)
	}

	// 4. editHistory形式に変換
	editHistory := make(map[string]*domain.FileEditHistory)
	for filePath, freq := range editFreqs {
		editHistory[filePath] = &domain.FileEditHistory{
			FilePath:   freq.FilePath,
			EditCount:  freq.EditCount,
			LastEdited: freq.LastEdited,
		}
	}

	// 5. 重要度スコアを計算
	config := domain.DefaultWeights()
	calculator := domain.NewImportanceCalculator(graph, editHistory, &config)

	scores, err := calculator.CalculateAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to calculate importance scores: %w", err)
	}

	// 6. スコアをDBに保存
	scoreMap := make(map[uuid.UUID]float64)
	for chunkID, score := range scores {
		scoreMap[chunkID] = score.FinalScore
	}

	if err := adapters.Index.BatchUpdateChunkImportanceScores(ctx, scoreMap); err != nil {
		return fmt.Errorf("failed to save importance scores: %w", err)
	}

	idx.logger.Info("Importance scores calculated and saved",
		"totalChunks", len(scoreMap),
		"graphNodes", len(graph.Nodes),
		"graphEdges", len(graph.Edges),
	)

	return nil
}

// findChunkIDByName は名前からチャンクIDを検索します（簡易版）
func (idx *Indexer) findChunkIDByName(chunks []*domain.Chunk, name string) *uuid.UUID {
	for _, chunk := range chunks {
		if chunk.Name != nil && *chunk.Name == name {
			return &chunk.ID
		}
	}
	return nil
}

// stringPtrOrEmpty はstring pointerを安全に文字列に変換します
func stringPtrOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// generateAndDisplayCoverageAlerts はカバレッジマップを構築してアラートを生成・表示します
func (idx *Indexer) generateAndDisplayCoverageAlerts(ctx context.Context, snapshotIDStr string, versionIdentifier string) {
	// スナップショットIDをパース
	snapshotID, err := uuid.Parse(snapshotIDStr)
	if err != nil {
		idx.logger.Warn("Failed to parse snapshot ID for coverage alerts", "snapshotID", snapshotIDStr, "error", err)
		return
	}

	// カバレッジマップを構築
	coverageBuilder := idx.createCoverageBuilder()
	coverageMap, err := coverageBuilder.BuildCoverageMap(ctx, snapshotID, versionIdentifier)
	if err != nil {
		idx.logger.Warn("Failed to build coverage map", "error", err)
		return
	}

	// アラートを生成
	alertGen := idx.createAlertGenerator()
	alerts, err := alertGen.GenerateAlerts(ctx, snapshotID, coverageMap)
	if err != nil {
		idx.logger.Warn("Failed to generate coverage alerts", "error", err)
		return
	}

	// アラートが存在しない場合は何もしない
	if len(alerts) == 0 {
		idx.logger.Info("No coverage alerts generated")
		return
	}

	// アラートを表示
	alertPrinter := idx.createAlertPrinter()
	alertPrinter.Print(alerts)

	// ログにも記録
	idx.logger.Info("Coverage alerts generated", "count", len(alerts))
	for _, alert := range alerts {
		idx.logger.Warn("Coverage alert",
			"severity", alert.Severity,
			"domain", alert.Domain,
			"message", alert.Message,
		)
	}
}

// createCoverageBuilder はCoverageBuilderを作成します
func (idx *Indexer) createCoverageBuilder() *coverage.CoverageBuilder {
	return coverage.NewCoverageBuilder(idx.indexReadRepo)
}

// createAlertGenerator はAlertGeneratorを作成します
func (idx *Indexer) createAlertGenerator() *coverage.AlertGenerator {
	return coverage.NewAlertGeneratorWithDefaults(idx.indexReadRepo)
}

// createAlertPrinter はAlertPrinterを作成します
func (idx *Indexer) createAlertPrinter() *coverage.AlertPrinter {
	// 標準出力にアラートを表示
	return coverage.NewAlertPrinter(os.Stdout)
}

// generateFileSummaries はすべてのファイルの要約を同期的に生成します
func (idx *Indexer) generateFileSummaries(ctx context.Context, files []*preparedFile) error {
	if idx.fileSummaryService == nil {
		idx.logger.Debug("FileSummaryService is not configured, skipping file summary generation")
		return nil
	}

	successCount := 0
	failureCount := 0

	for _, file := range files {
		// 言語情報とコンテンツが必要
		if file.Language == nil || file.Content == "" {
			idx.logger.Debug("Skipping file summary generation",
				"path", file.Path,
				"reason", "missing language or content",
			)
			continue
		}

		// ファイル要約を生成
		if err := idx.fileSummaryService.GenerateAndSave(
			ctx,
			file.FileID,
			file.Path,
			*file.Language,
			file.Content,
		); err != nil {
			idx.logger.Warn("Failed to generate file summary",
				"fileID", file.FileID,
				"filePath", file.Path,
				"error", err,
			)
			failureCount++
			continue
		}

		successCount++
	}

	idx.logger.Info("File summary generation completed",
		"success", successCount,
		"failure", failureCount,
		"total", len(files),
	)

	// すべて失敗した場合のみエラーを返す
	if successCount == 0 && failureCount > 0 {
		return fmt.Errorf("all file summary generations failed (%d failures)", failureCount)
	}

	return nil
}
