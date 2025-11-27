package ingestion

import (
	"context"
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
	repository     Repository
	sourceProvider SourceProvider
	embedder       Embedder
	llmClient      wiki.LLMClient // オプショナル
	chunkerFactory chunk.ChunkerFactory
	languageDetect chunk.LanguageDetector
	tokenCounter   chunk.TokenCounter
	chunkerConfig  *chunk.ChunkerConfig
	pipelineConfig *PipelineConfig
	logger         *slog.Logger
}

type indexServiceOptions struct {
	llmClient      wiki.LLMClient
	chunkerConfig  *chunk.ChunkerConfig
	pipelineConfig *PipelineConfig
	logger         *slog.Logger
}

// IndexServiceOption は IndexService のオプション設定
type IndexServiceOption func(*indexServiceOptions)

// WithIndexLogger は IndexService にロガーを設定する
func WithIndexLogger(logger *slog.Logger) IndexServiceOption {
	return func(o *indexServiceOptions) {
		o.logger = logger
	}
}

// WithIndexLLMClient は LLM クライアントを設定する
func WithIndexLLMClient(llm wiki.LLMClient) IndexServiceOption {
	return func(o *indexServiceOptions) {
		o.llmClient = llm
	}
}

// WithIndexChunkerConfig はチャンク設定を上書きする
func WithIndexChunkerConfig(cfg *chunk.ChunkerConfig) IndexServiceOption {
	return func(o *indexServiceOptions) {
		o.chunkerConfig = cfg
	}
}

// WithIndexPipelineConfig はパイプライン設定を上書きする
func WithIndexPipelineConfig(cfg *PipelineConfig) IndexServiceOption {
	return func(o *indexServiceOptions) {
		o.pipelineConfig = cfg
	}
}

// NewIndexService は新しいIndexServiceを作成する
func NewIndexService(
	repo Repository,
	sourceProvider SourceProvider,
	embedder Embedder,
	chunkerFactory chunk.ChunkerFactory,
	languageDetect chunk.LanguageDetector,
	tokenCounter chunk.TokenCounter,
	opts ...IndexServiceOption,
) *IndexService {
	options := indexServiceOptions{
		chunkerConfig:  chunk.DefaultChunkerConfig(),
		pipelineConfig: DefaultPipelineConfig(),
		logger:         slog.Default(),
	}
	for _, opt := range opts {
		opt(&options)
	}
	if options.logger == nil {
		options.logger = slog.Default()
	}
	if options.chunkerConfig == nil {
		options.chunkerConfig = chunk.DefaultChunkerConfig()
	}
	if options.pipelineConfig == nil {
		options.pipelineConfig = DefaultPipelineConfig()
	}

	return &IndexService{
		repository:     repo,
		sourceProvider: sourceProvider,
		embedder:       embedder,
		llmClient:      options.llmClient,
		chunkerFactory: chunkerFactory,
		languageDetect: languageDetect,
		tokenCounter:   tokenCounter,
		chunkerConfig:  options.chunkerConfig,
		pipelineConfig: options.pipelineConfig,
		logger:         options.logger,
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
		existingSnapshotOpt, err := s.repository.GetSnapshotByVersion(ctx, source.ID, versionIdentifier)
		if err == nil && existingSnapshotOpt.IsPresent() && existingSnapshotOpt.MustGet().Indexed {
			existingSnapshot := existingSnapshotOpt.MustGet()
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
			existingSnapshotOpt, getErr := s.repository.GetSnapshotByVersion(ctx, source.ID, versionIdentifier)
			if getErr != nil {
				return nil, fmt.Errorf("既存スナップショットの取得に失敗: %w", getErr)
			}
			if existingSnapshotOpt.IsAbsent() {
				return nil, fmt.Errorf("既存スナップショットが見つかりませんでした: %s", versionIdentifier)
			}
			existingSnapshot := existingSnapshotOpt.MustGet()
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

	// インデックス化コンテキストを作成
	docCtx := indexDocumentContext{
		ProductName:       params.ProductName,
		SourceName:        sourceName,
		VersionIdentifier: versionIdentifier,
	}

	// パイプライン処理でドキュメントをインデックス化
	pipeline := NewIndexPipeline(
		s.repository,
		s.embedder,
		s.chunkerFactory,
		s.languageDetect,
		s.pipelineConfig,
		s.logger,
	)

	processedFiles, totalChunks, err := pipeline.ProcessDocuments(
		ctx,
		snapshot.ID,
		documents,
		docCtx,
		s.sourceProvider.ShouldIgnore,
	)
	if err != nil {
		return nil, fmt.Errorf("パイプライン処理に失敗: %w", err)
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
