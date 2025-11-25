package search

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	searchdomain "github.com/jinford/dev-rag/internal/module/search/domain"
)

// HierarchicalSearchOptions は階層検索のオプションを表します
type HierarchicalSearchOptions struct {
	// IncludeParent が true の場合、検索結果の各チャンクの親チャンクを含めます
	IncludeParent bool

	// IncludeChildren が true の場合、検索結果の各チャンクの子チャンクを含めます
	IncludeChildren bool

	// IncludeAncestors が true の場合、検索結果の各チャンクの祖先チャンクを再帰的に含めます
	// (親、祖父母、曾祖父母など、ルートチャンクまで)
	IncludeAncestors bool

	// MaxDepth は階層を辿る最大深さを指定します（IncludeAncestors使用時）
	// 0 は無制限を意味します
	MaxDepth int
}

// HierarchicalSearchResult は階層情報を含む検索結果を表します
type HierarchicalSearchResult struct {
	// 元の検索結果
	*searchdomain.SearchResult

	// 親チャンク（存在しない場合は nil）
	Parent *searchdomain.ChunkContext

	// 子チャンクのリスト（存在しない場合は空スライス）
	Children []*searchdomain.ChunkContext

	// 祖先チャンクのリスト（IncludeAncestors が true の場合のみ）
	// 順序: [親, 祖父母, 曾祖父母, ...]（最も近い親から順に）
	Ancestors []*searchdomain.ChunkContext
}

// HierarchicalSearcher は階層検索機能を提供します
type HierarchicalSearcher struct {
	repo   searchdomain.ChunkContextReader
	logger *slog.Logger
}

// NewHierarchicalSearcher は階層検索用の構造体を生成します
func NewHierarchicalSearcher(repo searchdomain.ChunkContextReader, logger *slog.Logger) *HierarchicalSearcher {
	if repo == nil {
		panic("hierarchical_search.NewHierarchicalSearcher: repo is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &HierarchicalSearcher{
		repo:   repo,
		logger: logger,
	}
}

// EnrichWithHierarchy は検索結果に階層情報を追加します
func (h *HierarchicalSearcher) EnrichWithHierarchy(
	ctx context.Context,
	results []*searchdomain.SearchResult,
	options HierarchicalSearchOptions,
) ([]*HierarchicalSearchResult, error) {
	if len(results) == 0 {
		return []*HierarchicalSearchResult{}, nil
	}

	enriched := make([]*HierarchicalSearchResult, 0, len(results))

	for _, result := range results {
		enrichedResult := &HierarchicalSearchResult{
			SearchResult: result,
			Children:     []*searchdomain.ChunkContext{},
			Ancestors:    []*searchdomain.ChunkContext{},
		}

		// 親チャンクの取得
		if options.IncludeParent || options.IncludeAncestors {
			parent, err := h.repo.GetParentChunk(ctx, result.ChunkID)
			if err != nil {
				return nil, fmt.Errorf("failed to get parent chunk for %s: %w", result.ChunkID, err)
			}
			enrichedResult.Parent = parent
		}

		// 子チャンクの取得
		if options.IncludeChildren {
			children, err := h.repo.GetChildChunks(ctx, result.ChunkID)
			if err != nil {
				return nil, fmt.Errorf("failed to get child chunks for %s: %w", result.ChunkID, err)
			}
			enrichedResult.Children = children
		}

		// 祖先チャンクの再帰的取得
		if options.IncludeAncestors {
			ancestors, err := h.getAncestors(ctx, result.ChunkID, options.MaxDepth)
			if err != nil {
				return nil, fmt.Errorf("failed to get ancestors for %s: %w", result.ChunkID, err)
			}
			enrichedResult.Ancestors = ancestors
		}

		enriched = append(enriched, enrichedResult)
	}

	return enriched, nil
}

// GetParentChunk はチャンクIDから親チャンクを取得します
func (h *HierarchicalSearcher) GetParentChunk(ctx context.Context, chunkID uuid.UUID) (*searchdomain.ChunkContext, error) {
	parent, err := h.repo.GetParentChunk(ctx, chunkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent chunk: %w", err)
	}
	return parent, nil
}

// GetChildChunks はチャンクIDから子チャンクリストを取得します
func (h *HierarchicalSearcher) GetChildChunks(ctx context.Context, chunkID uuid.UUID) ([]*searchdomain.ChunkContext, error) {
	children, err := h.repo.GetChildChunks(ctx, chunkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get child chunks: %w", err)
	}
	return children, nil
}

// GetAncestors は祖先チャンクを再帰的に取得します
// 戻り値は [親, 祖父母, 曾祖父母, ...] の順序で返されます
func (h *HierarchicalSearcher) GetAncestors(ctx context.Context, chunkID uuid.UUID, maxDepth int) ([]*searchdomain.ChunkContext, error) {
	return h.getAncestors(ctx, chunkID, maxDepth)
}

// getAncestors は祖先チャンクを再帰的に取得する内部実装です
func (h *HierarchicalSearcher) getAncestors(ctx context.Context, chunkID uuid.UUID, maxDepth int) ([]*searchdomain.ChunkContext, error) {
	ancestors := make([]*searchdomain.ChunkContext, 0)
	visited := make(map[uuid.UUID]bool) // 循環参照を防止

	currentID := chunkID
	depth := 0

	for {
		// 最大深度チェック（0は無制限）
		if maxDepth > 0 && depth >= maxDepth {
			break
		}

		// 循環参照チェック
		if visited[currentID] {
			h.logger.Warn("circular reference detected in chunk hierarchy", "chunkID", currentID)
			break
		}
		visited[currentID] = true

		// 親チャンクを取得
		parent, err := h.repo.GetParentChunk(ctx, currentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent chunk: %w", err)
		}

		// 親が存在しない場合は終了
		if parent == nil {
			break
		}

		ancestors = append(ancestors, parent)
		currentID = parent.ID
		depth++
	}

	return ancestors, nil
}

// GetChunkTree はルートチャンクから階層ツリーを取得します
func (h *HierarchicalSearcher) GetChunkTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*searchdomain.ChunkContext, error) {
	tree, err := h.repo.GetChunkTree(ctx, rootID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk tree: %w", err)
	}
	return tree, nil
}

// BuildContextFromHierarchy は階層情報からLLMへのコンテキストテキストを構築します
// 親チャンク → 対象チャンク → 子チャンク の順序でテキストを構築します
func (h *HierarchicalSearcher) BuildContextFromHierarchy(result *HierarchicalSearchResult) string {
	var context string

	// 祖先チャンク（遠い順 → 近い順）
	if len(result.Ancestors) > 0 {
		context += "=== 上位コンテキスト ===\n\n"
		// 祖先は [親, 祖父母, ...] の順なので、逆順にする
		for i := len(result.Ancestors) - 1; i >= 0; i-- {
			ancestor := result.Ancestors[i]
			context += fmt.Sprintf("--- Level %d: %s ---\n", ancestor.Level, h.getChunkLabel(ancestor))
			context += ancestor.Content + "\n\n"
		}
	} else if result.Parent != nil {
		// Ancestors が無い場合は Parent のみ表示
		context += "=== 親コンテキスト ===\n\n"
		context += fmt.Sprintf("--- Level %d: %s ---\n", result.Parent.Level, h.getChunkLabel(result.Parent))
		context += result.Parent.Content + "\n\n"
	}

	// 対象チャンク
	context += "=== メインコンテンツ ===\n\n"
	context += result.Content + "\n\n"

	// 子チャンク
	if len(result.Children) > 0 {
		context += "=== 詳細コンテンツ（子チャンク） ===\n\n"
		for i, child := range result.Children {
			context += fmt.Sprintf("--- Level %d (%d/%d): %s ---\n", child.Level, i+1, len(result.Children), h.getChunkLabel(child))
			context += child.Content + "\n\n"
		}
	}

	return context
}

// getChunkLabel はチャンクのラベルを生成します（名前やタイプに基づく）
func (h *HierarchicalSearcher) getChunkLabel(chunk *searchdomain.ChunkContext) string {
	if chunk.Name != nil && *chunk.Name != "" {
		if chunk.Type != nil && *chunk.Type != "" {
			return fmt.Sprintf("%s %s", *chunk.Type, *chunk.Name)
		}
		return *chunk.Name
	}

	if chunk.Type != nil && *chunk.Type != "" {
		return *chunk.Type
	}

	return fmt.Sprintf("Chunk L%d-L%d", chunk.StartLine, chunk.EndLine)
}
