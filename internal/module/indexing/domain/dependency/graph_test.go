package dependency

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraph_AddNodeAndEdge(t *testing.T) {
	graph := NewGraph()

	// ノードを追加
	node1 := &Node{
		ChunkID:  uuid.New(),
		Name:     "funcA",
		Type:     "function",
		FilePath: "a.go",
	}
	node2 := &Node{
		ChunkID:  uuid.New(),
		Name:     "funcB",
		Type:     "function",
		FilePath: "b.go",
	}

	graph.AddNode(node1)
	graph.AddNode(node2)

	assert.Equal(t, 2, len(graph.Nodes))

	// エッジを追加
	edge := &Edge{
		From:         node1.ChunkID,
		To:           node2.ChunkID,
		RelationType: RelationTypeCalls,
		Weight:       1,
	}

	err := graph.AddEdge(edge)
	require.NoError(t, err)

	// エッジの確認
	outgoing := graph.GetOutgoingEdges(node1.ChunkID)
	assert.Equal(t, 1, len(outgoing))
	assert.Equal(t, node2.ChunkID, outgoing[0].To)

	incoming := graph.GetIncomingEdges(node2.ChunkID)
	assert.Equal(t, 1, len(incoming))
	assert.Equal(t, node1.ChunkID, incoming[0].From)
}

func TestGraph_GetDependencies(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "A", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "B", Type: "function"}
	node3 := &Node{ChunkID: uuid.New(), Name: "C", Type: "function"}

	graph.AddNode(node1)
	graph.AddNode(node2)
	graph.AddNode(node3)

	// A -> B, A -> C
	graph.AddEdge(&Edge{From: node1.ChunkID, To: node2.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node1.ChunkID, To: node3.ChunkID, RelationType: RelationTypeCalls, Weight: 1})

	deps := graph.GetDependencies(node1.ChunkID)
	assert.Equal(t, 2, len(deps))
}

func TestGraph_GetDependents(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "A", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "B", Type: "function"}
	node3 := &Node{ChunkID: uuid.New(), Name: "C", Type: "function"}

	graph.AddNode(node1)
	graph.AddNode(node2)
	graph.AddNode(node3)

	// B -> A, C -> A
	graph.AddEdge(&Edge{From: node2.ChunkID, To: node1.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node3.ChunkID, To: node1.ChunkID, RelationType: RelationTypeCalls, Weight: 1})

	dependents := graph.GetDependents(node1.ChunkID)
	assert.Equal(t, 2, len(dependents))
}

func TestGraph_GetReferenceCount(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "A", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "B", Type: "function"}
	node3 := &Node{ChunkID: uuid.New(), Name: "C", Type: "function"}

	graph.AddNode(node1)
	graph.AddNode(node2)
	graph.AddNode(node3)

	// B -> A, C -> A
	graph.AddEdge(&Edge{From: node2.ChunkID, To: node1.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node3.ChunkID, To: node1.ChunkID, RelationType: RelationTypeCalls, Weight: 1})

	refCount := graph.GetReferenceCount(node1.ChunkID)
	assert.Equal(t, 2, refCount)
}

func TestGraph_DetectCycles(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "A", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "B", Type: "function"}
	node3 := &Node{ChunkID: uuid.New(), Name: "C", Type: "function"}

	graph.AddNode(node1)
	graph.AddNode(node2)
	graph.AddNode(node3)

	// サイクル: A -> B -> C -> A
	graph.AddEdge(&Edge{From: node1.ChunkID, To: node2.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node2.ChunkID, To: node3.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node3.ChunkID, To: node1.ChunkID, RelationType: RelationTypeCalls, Weight: 1})

	cycles := graph.DetectCycles()
	assert.Greater(t, len(cycles), 0, "Cycle should be detected")
}

func TestGraph_NoCycles(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "A", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "B", Type: "function"}
	node3 := &Node{ChunkID: uuid.New(), Name: "C", Type: "function"}

	graph.AddNode(node1)
	graph.AddNode(node2)
	graph.AddNode(node3)

	// DAG: A -> B -> C
	graph.AddEdge(&Edge{From: node1.ChunkID, To: node2.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node2.ChunkID, To: node3.ChunkID, RelationType: RelationTypeCalls, Weight: 1})

	cycles := graph.DetectCycles()
	assert.Equal(t, 0, len(cycles), "No cycles should be detected")
}

func TestGraph_CalculateCentrality(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "A", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "B", Type: "function"}
	node3 := &Node{ChunkID: uuid.New(), Name: "C", Type: "function"}
	node4 := &Node{ChunkID: uuid.New(), Name: "D", Type: "function"}

	graph.AddNode(node1)
	graph.AddNode(node2)
	graph.AddNode(node3)
	graph.AddNode(node4)

	// B -> A, C -> A, A -> D
	graph.AddEdge(&Edge{From: node2.ChunkID, To: node1.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node3.ChunkID, To: node1.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node1.ChunkID, To: node4.ChunkID, RelationType: RelationTypeCalls, Weight: 1})

	// Aは最も中心的なノード（入次数2、出次数1）
	centralityA := graph.CalculateCentrality(node1.ChunkID)
	centralityB := graph.CalculateCentrality(node2.ChunkID)

	assert.Greater(t, centralityA, centralityB, "A should have higher centrality than B")
}

func TestGraph_TopologicalSort(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "A", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "B", Type: "function"}
	node3 := &Node{ChunkID: uuid.New(), Name: "C", Type: "function"}

	graph.AddNode(node1)
	graph.AddNode(node2)
	graph.AddNode(node3)

	// A -> B -> C
	graph.AddEdge(&Edge{From: node1.ChunkID, To: node2.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node2.ChunkID, To: node3.ChunkID, RelationType: RelationTypeCalls, Weight: 1})

	order, err := graph.GetTopologicalOrder()
	require.NoError(t, err)
	assert.Equal(t, 3, len(order))

	// Aが最初、Cが最後
	assert.Equal(t, node1.ChunkID, order[0].ChunkID)
	assert.Equal(t, node3.ChunkID, order[2].ChunkID)
}

func TestGraph_TopologicalSortWithCycle(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "A", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "B", Type: "function"}

	graph.AddNode(node1)
	graph.AddNode(node2)

	// サイクル: A -> B -> A
	graph.AddEdge(&Edge{From: node1.ChunkID, To: node2.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node2.ChunkID, To: node1.ChunkID, RelationType: RelationTypeCalls, Weight: 1})

	_, err := graph.GetTopologicalOrder()
	assert.Error(t, err, "Topological sort should fail with cycles")
}

func TestGraph_GetNodesByType(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "FuncA", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "FuncB", Type: "function"}
	node3 := &Node{ChunkID: uuid.New(), Name: "User", Type: "struct"}

	graph.AddNode(node1)
	graph.AddNode(node2)
	graph.AddNode(node3)

	functions := graph.GetNodesByType("function")
	assert.Equal(t, 2, len(functions))

	structs := graph.GetNodesByType("struct")
	assert.Equal(t, 1, len(structs))
}

func TestGraph_GetStats(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "A", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "B", Type: "function"}
	node3 := &Node{ChunkID: uuid.New(), Name: "C", Type: "function"}
	node4 := &Node{ChunkID: uuid.New(), Name: "D", Type: "function"}

	graph.AddNode(node1)
	graph.AddNode(node2)
	graph.AddNode(node3)
	graph.AddNode(node4)

	// A -> B, B -> C, C -> D
	graph.AddEdge(&Edge{From: node1.ChunkID, To: node2.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node2.ChunkID, To: node3.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node3.ChunkID, To: node4.ChunkID, RelationType: RelationTypeCalls, Weight: 1})

	stats := graph.GetStats()
	assert.Equal(t, 4, stats.NodeCount)
	assert.Equal(t, 3, stats.EdgeCount)
	assert.Equal(t, 0, stats.IsolatedNodes)
	assert.Equal(t, 0, stats.CycleCount)
}

func TestGraph_StronglyConnectedComponents(t *testing.T) {
	graph := NewGraph()

	node1 := &Node{ChunkID: uuid.New(), Name: "A", Type: "function"}
	node2 := &Node{ChunkID: uuid.New(), Name: "B", Type: "function"}
	node3 := &Node{ChunkID: uuid.New(), Name: "C", Type: "function"}
	node4 := &Node{ChunkID: uuid.New(), Name: "D", Type: "function"}

	graph.AddNode(node1)
	graph.AddNode(node2)
	graph.AddNode(node3)
	graph.AddNode(node4)

	// サイクル: A -> B -> C -> A, D is separate
	graph.AddEdge(&Edge{From: node1.ChunkID, To: node2.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node2.ChunkID, To: node3.ChunkID, RelationType: RelationTypeCalls, Weight: 1})
	graph.AddEdge(&Edge{From: node3.ChunkID, To: node1.ChunkID, RelationType: RelationTypeCalls, Weight: 1})

	sccs := graph.GetStronglyConnectedComponents()
	assert.Greater(t, len(sccs), 0, "Should find strongly connected components")

	// A, B, Cが1つのSCCを形成
	var foundLargeSCC bool
	for _, scc := range sccs {
		if len(scc) == 3 {
			foundLargeSCC = true
			break
		}
	}
	assert.True(t, foundLargeSCC, "Should find SCC with 3 nodes")
}
