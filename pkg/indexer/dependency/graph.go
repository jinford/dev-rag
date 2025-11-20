package dependency

import (
	"fmt"

	"github.com/google/uuid"
)

// Node はグラフのノード（チャンク）を表します
type Node struct {
	ChunkID  uuid.UUID
	Name     string   // 関数名、型名など
	Type     string   // function, struct, interface, etc.
	FilePath string
}

// Edge はグラフのエッジ（依存関係）を表します
type Edge struct {
	From         uuid.UUID
	To           uuid.UUID
	RelationType RelationType
	Weight       int // 依存の強さ（参照回数など）
}

// RelationType は依存関係の種類を表します
type RelationType int

const (
	RelationTypeUnknown RelationType = iota
	RelationTypeCalls                // 関数呼び出し
	RelationTypeImports              // インポート
	RelationTypeUses                 // 型使用
	RelationTypeExtends              // 継承（将来の拡張用）
	RelationTypeImplements           // インターフェース実装（将来の拡張用）
)

// Graph は依存関係グラフを表します
type Graph struct {
	Nodes map[uuid.UUID]*Node    // key: ChunkID
	Edges map[uuid.UUID][]*Edge  // key: From ChunkID
	// 逆引きインデックス（被参照関係）
	IncomingEdges map[uuid.UUID][]*Edge // key: To ChunkID
}

// NewGraph は新しいグラフを作成します
func NewGraph() *Graph {
	return &Graph{
		Nodes:         make(map[uuid.UUID]*Node),
		Edges:         make(map[uuid.UUID][]*Edge),
		IncomingEdges: make(map[uuid.UUID][]*Edge),
	}
}

// AddNode はノードを追加します
func (g *Graph) AddNode(node *Node) {
	g.Nodes[node.ChunkID] = node
}

// AddEdge はエッジを追加します
func (g *Graph) AddEdge(edge *Edge) error {
	// ノードの存在確認
	if _, ok := g.Nodes[edge.From]; !ok {
		return fmt.Errorf("from node not found: %s", edge.From)
	}
	if _, ok := g.Nodes[edge.To]; !ok {
		return fmt.Errorf("to node not found: %s", edge.To)
	}

	// エッジを追加
	g.Edges[edge.From] = append(g.Edges[edge.From], edge)
	g.IncomingEdges[edge.To] = append(g.IncomingEdges[edge.To], edge)

	return nil
}

// GetOutgoingEdges は指定ノードから出ているエッジを取得します
func (g *Graph) GetOutgoingEdges(chunkID uuid.UUID) []*Edge {
	if edges, ok := g.Edges[chunkID]; ok {
		return edges
	}
	return make([]*Edge, 0)
}

// GetIncomingEdges は指定ノードへ入ってくるエッジを取得します
func (g *Graph) GetIncomingEdges(chunkID uuid.UUID) []*Edge {
	if edges, ok := g.IncomingEdges[chunkID]; ok {
		return edges
	}
	return make([]*Edge, 0)
}

// GetDependencies は指定チャンクが依存しているチャンクのリストを取得します
func (g *Graph) GetDependencies(chunkID uuid.UUID) []*Node {
	edges := g.GetOutgoingEdges(chunkID)
	nodes := make([]*Node, 0, len(edges))

	for _, edge := range edges {
		if node, ok := g.Nodes[edge.To]; ok {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

// GetDependents は指定チャンクに依存しているチャンクのリストを取得します
func (g *Graph) GetDependents(chunkID uuid.UUID) []*Node {
	edges := g.GetIncomingEdges(chunkID)
	nodes := make([]*Node, 0, len(edges))

	for _, edge := range edges {
		if node, ok := g.Nodes[edge.From]; ok {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

// GetReferenceCount は指定チャンクの被参照回数を取得します
func (g *Graph) GetReferenceCount(chunkID uuid.UUID) int {
	return len(g.GetIncomingEdges(chunkID))
}

// DetectCycles は循環依存を検出します
func (g *Graph) DetectCycles() [][]*Node {
	cycles := make([][]*Node, 0)
	visited := make(map[uuid.UUID]bool)
	recursionStack := make(map[uuid.UUID]bool)
	path := make([]*Node, 0)

	for chunkID := range g.Nodes {
		if !visited[chunkID] {
			if cycle := g.detectCyclesDFS(chunkID, visited, recursionStack, path); cycle != nil {
				cycles = append(cycles, cycle)
			}
		}
	}

	return cycles
}

// detectCyclesDFS は深さ優先探索で循環依存を検出します
func (g *Graph) detectCyclesDFS(chunkID uuid.UUID, visited, recursionStack map[uuid.UUID]bool, path []*Node) []*Node {
	visited[chunkID] = true
	recursionStack[chunkID] = true
	path = append(path, g.Nodes[chunkID])

	// 隣接ノードを探索
	edges := g.GetOutgoingEdges(chunkID)
	for _, edge := range edges {
		nextID := edge.To

		if !visited[nextID] {
			if cycle := g.detectCyclesDFS(nextID, visited, recursionStack, path); cycle != nil {
				return cycle
			}
		} else if recursionStack[nextID] {
			// 循環を検出
			cycle := make([]*Node, 0)
			foundStart := false
			for _, node := range path {
				if node.ChunkID == nextID {
					foundStart = true
				}
				if foundStart {
					cycle = append(cycle, node)
				}
			}
			cycle = append(cycle, g.Nodes[nextID]) // サイクルを閉じる
			return cycle
		}
	}

	recursionStack[chunkID] = false
	return nil
}

// CalculateCentrality は簡易的な中心性スコアを計算します
// (入次数 + 出次数) / 2を正規化
func (g *Graph) CalculateCentrality(chunkID uuid.UUID) float64 {
	if _, ok := g.Nodes[chunkID]; !ok {
		return 0.0
	}

	inDegree := len(g.GetIncomingEdges(chunkID))
	outDegree := len(g.GetOutgoingEdges(chunkID))

	// 最大次数を計算（正規化のため）
	maxDegree := 0
	for id := range g.Nodes {
		degree := len(g.GetIncomingEdges(id)) + len(g.GetOutgoingEdges(id))
		if degree > maxDegree {
			maxDegree = degree
		}
	}

	if maxDegree == 0 {
		return 0.0
	}

	return float64(inDegree+outDegree) / float64(maxDegree)
}

// GetStronglyConnectedComponents は強連結成分を取得します（Tarjanのアルゴリズム）
func (g *Graph) GetStronglyConnectedComponents() [][]*Node {
	index := 0
	stack := make([]*Node, 0)
	indices := make(map[uuid.UUID]int)
	lowlinks := make(map[uuid.UUID]int)
	onStack := make(map[uuid.UUID]bool)
	sccs := make([][]*Node, 0)

	var strongConnect func(uuid.UUID)
	strongConnect = func(chunkID uuid.UUID) {
		indices[chunkID] = index
		lowlinks[chunkID] = index
		index++
		stack = append(stack, g.Nodes[chunkID])
		onStack[chunkID] = true

		// 後続ノードを探索
		edges := g.GetOutgoingEdges(chunkID)
		for _, edge := range edges {
			nextID := edge.To
			if _, ok := indices[nextID]; !ok {
				strongConnect(nextID)
				if lowlinks[nextID] < lowlinks[chunkID] {
					lowlinks[chunkID] = lowlinks[nextID]
				}
			} else if onStack[nextID] {
				if indices[nextID] < lowlinks[chunkID] {
					lowlinks[chunkID] = indices[nextID]
				}
			}
		}

		// SCCのルートの場合
		if lowlinks[chunkID] == indices[chunkID] {
			scc := make([]*Node, 0)
			for {
				node := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[node.ChunkID] = false
				scc = append(scc, node)
				if node.ChunkID == chunkID {
					break
				}
			}
			if len(scc) > 1 { // 自己ループでない強連結成分のみ追加
				sccs = append(sccs, scc)
			}
		}
	}

	for chunkID := range g.Nodes {
		if _, ok := indices[chunkID]; !ok {
			strongConnect(chunkID)
		}
	}

	return sccs
}

// GetTopologicalOrder はトポロジカルソートを実行します
// 循環がある場合はエラーを返します
func (g *Graph) GetTopologicalOrder() ([]*Node, error) {
	inDegree := make(map[uuid.UUID]int)
	result := make([]*Node, 0)

	// 各ノードの入次数を計算
	for chunkID := range g.Nodes {
		inDegree[chunkID] = len(g.GetIncomingEdges(chunkID))
	}

	// 入次数0のノードをキューに追加
	queue := make([]uuid.UUID, 0)
	for chunkID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, chunkID)
		}
	}

	// トポロジカルソート
	for len(queue) > 0 {
		chunkID := queue[0]
		queue = queue[1:]
		result = append(result, g.Nodes[chunkID])

		// 隣接ノードの入次数を減らす
		edges := g.GetOutgoingEdges(chunkID)
		for _, edge := range edges {
			nextID := edge.To
			inDegree[nextID]--
			if inDegree[nextID] == 0 {
				queue = append(queue, nextID)
			}
		}
	}

	// すべてのノードが処理されたかチェック
	if len(result) != len(g.Nodes) {
		return nil, fmt.Errorf("cycle detected: cannot perform topological sort")
	}

	return result, nil
}

// GetNodesByType は指定された型のノードをすべて取得します
func (g *Graph) GetNodesByType(nodeType string) []*Node {
	nodes := make([]*Node, 0)
	for _, node := range g.Nodes {
		if node.Type == nodeType {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetStats はグラフの統計情報を取得します
type GraphStats struct {
	NodeCount      int
	EdgeCount      int
	AvgInDegree    float64
	AvgOutDegree   float64
	MaxInDegree    int
	MaxOutDegree   int
	CycleCount     int
	IsolatedNodes  int
}

func (g *Graph) GetStats() *GraphStats {
	stats := &GraphStats{
		NodeCount: len(g.Nodes),
	}

	totalInDegree := 0
	totalOutDegree := 0
	isolatedNodes := 0

	for chunkID := range g.Nodes {
		inDegree := len(g.GetIncomingEdges(chunkID))
		outDegree := len(g.GetOutgoingEdges(chunkID))

		totalInDegree += inDegree
		totalOutDegree += outDegree

		if inDegree > stats.MaxInDegree {
			stats.MaxInDegree = inDegree
		}
		if outDegree > stats.MaxOutDegree {
			stats.MaxOutDegree = outDegree
		}

		if inDegree == 0 && outDegree == 0 {
			isolatedNodes++
		}

		stats.EdgeCount += outDegree
	}

	if len(g.Nodes) > 0 {
		stats.AvgInDegree = float64(totalInDegree) / float64(len(g.Nodes))
		stats.AvgOutDegree = float64(totalOutDegree) / float64(len(g.Nodes))
	}

	stats.IsolatedNodes = isolatedNodes
	stats.CycleCount = len(g.DetectCycles())

	return stats
}
