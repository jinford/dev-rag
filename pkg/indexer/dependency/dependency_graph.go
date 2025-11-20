package dependency

import (
	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/repository"
)

// DependencyType は依存関係の種類を表します
type DependencyType string

const (
	DependencyTypeCall   DependencyType = "call"   // 関数呼び出し
	DependencyTypeImport DependencyType = "import" // インポート
	DependencyTypeType   DependencyType = "type"   // 型依存
)

// Dependency は依存関係のエッジを表します
type Dependency struct {
	FromChunkID uuid.UUID
	ToChunkID   uuid.UUID
	Type        DependencyType
	Symbol      string // 依存の対象シンボル（関数名、型名など）
}

// DependencyGraph は依存関係グラフを表します
type DependencyGraph struct {
	// adjacencyList[chunkID] = そのチャンクが依存している他のチャンクのリスト
	adjacencyList map[uuid.UUID][]*Dependency
	// reverseAdjacencyList[chunkID] = そのチャンクに依存している他のチャンクのリスト
	reverseAdjacencyList map[uuid.UUID][]*Dependency
}

// NewDependencyGraph は新しい依存グラフを作成します
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		adjacencyList:        make(map[uuid.UUID][]*Dependency),
		reverseAdjacencyList: make(map[uuid.UUID][]*Dependency),
	}
}

// AddDependency は依存関係を追加します
func (g *DependencyGraph) AddDependency(dep *Dependency) {
	g.adjacencyList[dep.FromChunkID] = append(g.adjacencyList[dep.FromChunkID], dep)
	g.reverseAdjacencyList[dep.ToChunkID] = append(g.reverseAdjacencyList[dep.ToChunkID], dep)
}

// GetOutgoingDependencies は指定されたチャンクの出次数（依存先）を取得します
func (g *DependencyGraph) GetOutgoingDependencies(chunkID uuid.UUID) []*Dependency {
	return g.adjacencyList[chunkID]
}

// GetIncomingDependencies は指定されたチャンクの入次数（被依存）を取得します
func (g *DependencyGraph) GetIncomingDependencies(chunkID uuid.UUID) []*Dependency {
	return g.reverseAdjacencyList[chunkID]
}

// GetReferenceCount は指定されたチャンクの被参照回数を返します
func (g *DependencyGraph) GetReferenceCount(chunkID uuid.UUID) int {
	return len(g.reverseAdjacencyList[chunkID])
}

// DetectCycles は循環依存を検出します
func (g *DependencyGraph) DetectCycles() [][]uuid.UUID {
	visited := make(map[uuid.UUID]bool)
	recStack := make(map[uuid.UUID]bool)
	cycles := [][]uuid.UUID{}

	var dfs func(uuid.UUID, []uuid.UUID) bool
	dfs = func(node uuid.UUID, path []uuid.UUID) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, dep := range g.adjacencyList[node] {
			if !visited[dep.ToChunkID] {
				if dfs(dep.ToChunkID, path) {
					return true
				}
			} else if recStack[dep.ToChunkID] {
				// 循環検出
				cycleStart := -1
				for i, id := range path {
					if id == dep.ToChunkID {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycles = append(cycles, path[cycleStart:])
				}
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for node := range g.adjacencyList {
		if !visited[node] {
			dfs(node, []uuid.UUID{})
		}
	}

	return cycles
}

// ChunkWithMetadata はチャンクとそのメタデータを保持します
type ChunkWithMetadata struct {
	ChunkID  uuid.UUID
	Metadata *repository.ChunkMetadata
}

// BuildFromChunks はチャンクリストから依存グラフを構築します
func BuildFromChunks(chunks []*ChunkWithMetadata) (*DependencyGraph, error) {
	graph := NewDependencyGraph()

	// チャンク名からIDへのマッピングを構築
	nameToID := make(map[string]uuid.UUID)
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata.Name != nil {
			nameToID[*chunk.Metadata.Name] = chunk.ChunkID
		}
	}

	// 各チャンクの依存関係をグラフに追加
	for _, chunk := range chunks {
		if chunk.Metadata == nil {
			continue
		}

		// 関数呼び出しからの依存関係
		for _, call := range chunk.Metadata.Calls {
			if toID, exists := nameToID[call]; exists {
				graph.AddDependency(&Dependency{
					FromChunkID: chunk.ChunkID,
					ToChunkID:   toID,
					Type:        DependencyTypeCall,
					Symbol:      call,
				})
			}
		}

		// 型依存
		for _, typeDep := range chunk.Metadata.TypeDependencies {
			if toID, exists := nameToID[typeDep]; exists {
				graph.AddDependency(&Dependency{
					FromChunkID: chunk.ChunkID,
					ToChunkID:   toID,
					Type:        DependencyTypeType,
					Symbol:      typeDep,
				})
			}
		}
	}

	return graph, nil
}
