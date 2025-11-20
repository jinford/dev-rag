package dependency

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_RealWorldExample は実際のGoコードでの統合テストです
func TestIntegration_RealWorldExample(t *testing.T) {
	source := `package indexer

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/repository"
)

type Indexer struct {
	repo   *repository.IndexRepositoryRW
	chunker *Chunker
}

func NewIndexer(repo *repository.IndexRepositoryRW) *Indexer {
	return &Indexer{
		repo:   repo,
		chunker: NewChunker(),
	}
}

func (i *Indexer) IndexFile(ctx context.Context, filePath string, content string) error {
	chunks := i.chunker.Chunk(content)

	for _, chunk := range chunks {
		chunkID := uuid.New()
		if err := i.repo.CreateChunk(ctx, chunkID, chunk); err != nil {
			return fmt.Errorf("failed to create chunk: %w", err)
		}
	}

	return nil
}

func (i *Indexer) GetChunk(ctx context.Context, id uuid.UUID) (*models.Chunk, error) {
	return i.repo.GetChunkByID(ctx, id)
}
`

	// go.modデータを準備
	goModData := map[string]string{
		"github.com/google/uuid":            "v1.6.0",
		"github.com/jinford/dev-rag/pkg/models": "v0.1.0",
		"github.com/jinford/dev-rag/pkg/repository": "v0.1.0",
	}

	// 解析実行
	analyzer := NewAnalyzer()
	info, err := analyzer.Analyze(source, goModData)
	require.NoError(t, err)
	require.NotNil(t, info)

	// インポート情報の検証
	t.Run("Imports", func(t *testing.T) {
		assert.Equal(t, 5, len(info.Imports), "Should have 5 imports")

		// 標準ライブラリ
		assert.Equal(t, ImportTypeStandard, info.Imports["context"].Type)
		assert.Equal(t, ImportTypeStandard, info.Imports["fmt"].Type)

		// 外部依存
		assert.Equal(t, ImportTypeExternal, info.Imports["github.com/google/uuid"].Type)
		assert.Equal(t, "v1.6.0", info.Imports["github.com/google/uuid"].Version)

		// 内部パッケージ
		assert.Equal(t, ImportTypeInternal, info.Imports["github.com/jinford/dev-rag/pkg/models"].Type)
		assert.Equal(t, ImportTypeInternal, info.Imports["github.com/jinford/dev-rag/pkg/repository"].Type)
	})

	// 関数呼び出しの検証
	t.Run("FunctionCalls", func(t *testing.T) {
		assert.Greater(t, len(info.FunctionCalls), 0, "Should have function calls")

		// uuid.New() の呼び出しを確認
		var foundUUIDNew bool
		for _, call := range info.FunctionCalls {
			if call.Name == "New" && call.Package == "uuid" {
				foundUUIDNew = true
				break
			}
		}
		assert.True(t, foundUUIDNew, "Should find uuid.New() call")

		// fmt.Errorf の呼び出しを確認
		var foundErrorf bool
		for _, call := range info.FunctionCalls {
			if call.Name == "Errorf" && call.Package == "fmt" {
				foundErrorf = true
				break
			}
		}
		assert.True(t, foundErrorf, "Should find fmt.Errorf call")
	})

	// 型依存の検証
	t.Run("TypeDependencies", func(t *testing.T) {
		assert.Contains(t, info.TypeDeps, "Indexer", "Should have Indexer type")

		indexerType := info.TypeDeps["Indexer"]
		assert.Equal(t, TypeKindStruct, indexerType.Kind)

		// フィールドの型を確認
		assert.Contains(t, indexerType.FieldTypes, "*Chunker")
	})
}

// TestIntegration_DependencyGraph は依存グラフの統合テストです
func TestIntegration_DependencyGraph(t *testing.T) {
	// 複数のファイルを模擬したグラフを構築
	graph := NewGraph()

	// ノードを作成
	mainFunc := &Node{
		ChunkID:  uuid.New(),
		Name:     "main",
		Type:     "function",
		FilePath: "main.go",
	}
	indexerFunc := &Node{
		ChunkID:  uuid.New(),
		Name:     "IndexFile",
		Type:     "method",
		FilePath: "indexer.go",
	}
	chunkerFunc := &Node{
		ChunkID:  uuid.New(),
		Name:     "Chunk",
		Type:     "method",
		FilePath: "chunker.go",
	}
	repoFunc := &Node{
		ChunkID:  uuid.New(),
		Name:     "CreateChunk",
		Type:     "method",
		FilePath: "repository.go",
	}
	utilFunc := &Node{
		ChunkID:  uuid.New(),
		Name:     "ValidateInput",
		Type:     "function",
		FilePath: "util.go",
	}

	graph.AddNode(mainFunc)
	graph.AddNode(indexerFunc)
	graph.AddNode(chunkerFunc)
	graph.AddNode(repoFunc)
	graph.AddNode(utilFunc)

	// 依存関係を追加
	// main -> IndexFile -> Chunk -> CreateChunk
	//                   -> ValidateInput
	graph.AddEdge(&Edge{
		From:         mainFunc.ChunkID,
		To:           indexerFunc.ChunkID,
		RelationType: RelationTypeCalls,
		Weight:       1,
	})
	graph.AddEdge(&Edge{
		From:         indexerFunc.ChunkID,
		To:           chunkerFunc.ChunkID,
		RelationType: RelationTypeCalls,
		Weight:       1,
	})
	graph.AddEdge(&Edge{
		From:         indexerFunc.ChunkID,
		To:           repoFunc.ChunkID,
		RelationType: RelationTypeCalls,
		Weight:       3, // 複数回呼ばれる
	})
	graph.AddEdge(&Edge{
		From:         indexerFunc.ChunkID,
		To:           utilFunc.ChunkID,
		RelationType: RelationTypeCalls,
		Weight:       1,
	})

	// グラフ解析
	t.Run("BasicQueries", func(t *testing.T) {
		// IndexFileが依存している関数
		deps := graph.GetDependencies(indexerFunc.ChunkID)
		assert.Equal(t, 3, len(deps), "IndexFile should depend on 3 functions")

		// IndexFileに依存している関数
		dependents := graph.GetDependents(indexerFunc.ChunkID)
		assert.Equal(t, 1, len(dependents), "IndexFile should be depended by 1 function")

		// 被参照回数
		refCount := graph.GetReferenceCount(indexerFunc.ChunkID)
		assert.Equal(t, 1, refCount)

		refCountRepo := graph.GetReferenceCount(repoFunc.ChunkID)
		assert.Equal(t, 1, refCountRepo)
	})

	t.Run("Centrality", func(t *testing.T) {
		// IndexFileは中心的なノード
		centralityIndexer := graph.CalculateCentrality(indexerFunc.ChunkID)
		centralityUtil := graph.CalculateCentrality(utilFunc.ChunkID)

		assert.Greater(t, centralityIndexer, centralityUtil,
			"IndexFile should have higher centrality than util")
	})

	t.Run("TopologicalOrder", func(t *testing.T) {
		order, err := graph.GetTopologicalOrder()
		require.NoError(t, err)
		assert.Equal(t, 5, len(order))

		// mainが最初、CreateChunkが最後付近
		assert.Equal(t, mainFunc.ChunkID, order[0].ChunkID)
	})

	t.Run("Statistics", func(t *testing.T) {
		stats := graph.GetStats()
		assert.Equal(t, 5, stats.NodeCount)
		assert.Equal(t, 4, stats.EdgeCount)
		assert.Equal(t, 0, stats.CycleCount, "Should have no cycles")
		assert.Equal(t, 0, stats.IsolatedNodes, "Should have no isolated nodes")
	})
}

// TestIntegration_CyclicDependencies は循環依存の検出テストです
func TestIntegration_CyclicDependencies(t *testing.T) {
	graph := NewGraph()

	// サービスAがサービスBに依存し、サービスBがサービスAに依存する
	serviceA := &Node{
		ChunkID:  uuid.New(),
		Name:     "ServiceA",
		Type:     "struct",
		FilePath: "service_a.go",
	}
	serviceB := &Node{
		ChunkID:  uuid.New(),
		Name:     "ServiceB",
		Type:     "struct",
		FilePath: "service_b.go",
	}
	serviceC := &Node{
		ChunkID:  uuid.New(),
		Name:     "ServiceC",
		Type:     "struct",
		FilePath: "service_c.go",
	}

	graph.AddNode(serviceA)
	graph.AddNode(serviceB)
	graph.AddNode(serviceC)

	// サイクル: A -> B -> C -> A
	graph.AddEdge(&Edge{
		From:         serviceA.ChunkID,
		To:           serviceB.ChunkID,
		RelationType: RelationTypeUses,
		Weight:       1,
	})
	graph.AddEdge(&Edge{
		From:         serviceB.ChunkID,
		To:           serviceC.ChunkID,
		RelationType: RelationTypeUses,
		Weight:       1,
	})
	graph.AddEdge(&Edge{
		From:         serviceC.ChunkID,
		To:           serviceA.ChunkID,
		RelationType: RelationTypeUses,
		Weight:       1,
	})

	// 循環依存を検出
	cycles := graph.DetectCycles()
	assert.Greater(t, len(cycles), 0, "Should detect cycles")

	// SCCを取得
	sccs := graph.GetStronglyConnectedComponents()
	assert.Greater(t, len(sccs), 0, "Should find strongly connected components")

	// トポロジカルソートは失敗するはず
	_, err := graph.GetTopologicalOrder()
	assert.Error(t, err, "Topological sort should fail with cycles")
}

// TestIntegration_ComplexProject は複雑なプロジェクト構造のテストです
func TestIntegration_ComplexProject(t *testing.T) {
	source := `package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// User はユーザー情報を表します
type User struct {
	ID        uuid.UUID
	Name      string
	Email     string
	CreatedAt time.Time
}

// UserRepository はユーザーのリポジトリです
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository は新しいUserRepositoryを作成します
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetUser はIDでユーザーを取得します
func (r *UserRepository) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	query := "SELECT id, name, email, created_at FROM users WHERE id = $1"

	var user User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// SaveUser はユーザーを保存します
func (r *UserRepository) SaveUser(ctx context.Context, user *User) error {
	query := "INSERT INTO users (id, name, email, created_at) VALUES ($1, $2, $3, $4)"

	_, err := r.db.ExecContext(ctx, query, user.ID, user.Name, user.Email, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}

	return nil
}

// UserService はユーザーのビジネスロジックを提供します
type UserService struct {
	repo *UserRepository
}

// CreateUser は新しいユーザーを作成します
func (s *UserService) CreateUser(ctx context.Context, name, email string) (*User, error) {
	user := &User{
		ID:        uuid.New(),
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
	}

	if err := s.repo.SaveUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// HandleCreateUser はHTTPハンドラです
func HandleCreateUser(service *UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.FormValue("name")
		email := r.FormValue("email")

		user, err := service.CreateUser(r.Context(), name, email)
		if err != nil {
			log.Printf("Error creating user: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "User created: %s", user.ID)
	}
}
`

	goModData := map[string]string{
		"github.com/google/uuid":  "v1.6.0",
		"github.com/jackc/pgx/v5": "v5.7.6",
	}

	analyzer := NewAnalyzer()
	info, err := analyzer.Analyze(source, goModData)
	require.NoError(t, err)

	t.Run("CompleteAnalysis", func(t *testing.T) {
		// インポート数の確認
		assert.Greater(t, len(info.Imports), 5, "Should have multiple imports")

		// 型定義の確認
		assert.Contains(t, info.TypeDeps, "User")
		assert.Contains(t, info.TypeDeps, "UserRepository")
		assert.Contains(t, info.TypeDeps, "UserService")

		// User構造体のフィールド
		userType := info.TypeDeps["User"]
		assert.Equal(t, TypeKindStruct, userType.Kind)
		assert.Contains(t, userType.FieldTypes, "string")

		// 関数呼び出しの確認
		var foundUUIDNew bool
		var foundTimeNow bool
		for _, call := range info.FunctionCalls {
			if call.Name == "New" && call.Package == "uuid" {
				foundUUIDNew = true
			}
			if call.Name == "Now" && call.Package == "time" {
				foundTimeNow = true
			}
		}
		assert.True(t, foundUUIDNew, "Should find uuid.New()")
		assert.True(t, foundTimeNow, "Should find time.Now()")
	})
}
