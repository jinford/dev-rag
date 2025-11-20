package query_test

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/query"
)

// Example_hierarchicalSearchWithContextBuilder は階層検索とコンテキスト構築の基本的な使用例を示します
func Example_hierarchicalSearchWithContextBuilder() {
	// この例では、階層検索の結果をLLMに送信するためのコンテキストに整形する方法を示します

	// 1. 検索結果を準備（実際にはsearch.Searcherから取得）
	baseResults := []*models.SearchResult{
		{
			ChunkID:   uuid.New(),
			FilePath:  "pkg/auth/handler.go",
			StartLine: 10,
			EndLine:   20,
			Content:   "func HandleLogin(w http.ResponseWriter, r *http.Request) { ... }",
			Score:     0.95,
		},
	}

	// 2. Querierを使用して階層情報を追加
	// （実際にはrepository.IndexRepositoryRを使用）
	// querier := query.NewQuerier(indexRepo)
	//
	// options := &query.SearchOptions{
	//     IncludeParent:   true,
	//     IncludeChildren: true,
	//     MaxDepth:        1,
	// }
	//
	// enrichedResults, err := querier.EnrichSearchResultsWithHierarchy(ctx, baseResults, options)
	// if err != nil {
	//     panic(err)
	// }

	// 3. ContextBuilderでLLMコンテキストを構築
	contextBuilder := query.NewContextBuilder(8000) // 8000トークン

	// 実際の使用例（モックデータ）
	enrichedResults := []*query.EnhancedSearchResult{
		{
			SearchResult: baseResults[0],
			ParentChunk: &models.Chunk{
				Content: "package auth\n\ntype AuthHandler struct { ... }",
			},
			ChildChunks: []*models.Chunk{
				{Content: "// ログイン処理の実装"},
			},
		},
	}

	llmContext := contextBuilder.BuildContextWithHierarchy(enrichedResults)
	llmContext = contextBuilder.TruncateToTokenLimit(llmContext)

	fmt.Println("LLMコンテキストが構築されました")
	fmt.Printf("推定トークン数: %d\n", contextBuilder.EstimateTokenCount(llmContext))

	// Output:
	// LLMコンテキストが構築されました
	// 推定トークン数: 73
}

// Example_compactContext はコンパクトなコンテキスト構築の使用例を示します
func Example_compactContext() {
	// トークン制限が厳しい場合の使用例

	baseResults := []*models.SearchResult{
		{
			ChunkID:   uuid.New(),
			FilePath:  "main.go",
			StartLine: 1,
			EndLine:   10,
			Content:   "package main\n\nfunc main() { ... }",
			Score:     0.9,
		},
	}

	enrichedResults := []*query.EnhancedSearchResult{
		{SearchResult: baseResults[0]},
	}

	contextBuilder := query.NewContextBuilder(2000) // トークン制限: 2000
	compactContext := contextBuilder.BuildCompactContext(enrichedResults)

	fmt.Println("コンパクトなコンテキストが構築されました")
	fmt.Printf("推定トークン数: %d\n", contextBuilder.EstimateTokenCount(compactContext))

	// Output:
	// コンパクトなコンテキストが構築されました
	// 推定トークン数: 13
}

// Example_metadataContext はメタデータを含むコンテキスト構築の使用例を示します
func Example_metadataContext() {
	// スコアやファイルパスなどの詳細情報を含める場合の使用例

	baseResults := []*models.SearchResult{
		{
			ChunkID:   uuid.New(),
			FilePath:  "pkg/service/user.go",
			StartLine: 50,
			EndLine:   60,
			Content:   "func GetUser(id string) (*User, error) { ... }",
			Score:     0.88,
		},
	}

	enrichedResults := []*query.EnhancedSearchResult{
		{SearchResult: baseResults[0]},
	}

	contextBuilder := query.NewContextBuilder(8000)
	metadataContext := contextBuilder.BuildContextWithMetadata(enrichedResults)

	fmt.Println("メタデータを含むコンテキストが構築されました")
	fmt.Printf("推定トークン数: %d\n", contextBuilder.EstimateTokenCount(metadataContext))

	// Output:
	// メタデータを含むコンテキストが構築されました
	// 推定トークン数: 46
}

// Example_parentAndChildrenContext は親チャンクと子チャンクを含むコンテキスト構築の使用例を示します
func Example_parentAndChildrenContext() {
	ctx := context.Background()

	// 1. 検索結果
	baseResults := []*models.SearchResult{
		{
			ChunkID:   uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
			FilePath:  "pkg/handler/user.go",
			StartLine: 20,
			EndLine:   40,
			Content:   "func CreateUser(req CreateUserRequest) (*User, error) { ... }",
			Score:     0.92,
		},
	}

	// 2. 階層情報を追加（実際にはQuerierを使用）
	// 親チャンク: ファイルレベルのコンテキスト
	parentChunk := &models.Chunk{
		Content:   "package handler\n\ntype UserHandler struct { service *UserService }",
		StartLine: 1,
		EndLine:   10,
	}

	// 子チャンク: 関数内の詳細な実装
	childChunks := []*models.Chunk{
		{
			Content:   "// バリデーション処理",
			StartLine: 21,
			EndLine:   25,
		},
		{
			Content:   "// ユーザー作成処理",
			StartLine: 26,
			EndLine:   35,
		},
	}

	enrichedResults := []*query.EnhancedSearchResult{
		{
			SearchResult: baseResults[0],
			ParentChunk:  parentChunk,
			ChildChunks:  childChunks,
		},
	}

	// 3. コンテキスト構築
	contextBuilder := query.NewContextBuilder(8000)
	llmContext := contextBuilder.BuildContextWithHierarchy(enrichedResults)

	fmt.Printf("コンテキスト: %d 文字\n", len(llmContext))
	fmt.Printf("推定トークン数: %d\n", contextBuilder.EstimateTokenCount(llmContext))

	// 親チャンクと子チャンクが含まれることを確認
	_ = ctx // 使用しない変数の警告を回避

	// Output:
	// コンテキスト: 359 文字
	// 推定トークン数: 89
}
