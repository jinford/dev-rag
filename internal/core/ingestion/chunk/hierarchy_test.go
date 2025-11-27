package chunk

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/mo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHierarchyRepository はテスト用のモックリポジトリです
type mockHierarchyRepository struct {
	relations map[uuid.UUID][]uuid.UUID // parentID -> []childID
	ordinals  map[string]int            // "parentID:childID" -> ordinal
}

func newMockHierarchyRepository() *mockHierarchyRepository {
	return &mockHierarchyRepository{
		relations: make(map[uuid.UUID][]uuid.UUID),
		ordinals:  make(map[string]int),
	}
}

func (m *mockHierarchyRepository) AddChunkRelation(ctx context.Context, parentID, childID uuid.UUID, ordinal int) error {
	// 既に同じ関係が存在するかチェック
	for _, existingChild := range m.relations[parentID] {
		if existingChild == childID {
			return fmt.Errorf("relation already exists: %s -> %s", parentID, childID)
		}
	}

	m.relations[parentID] = append(m.relations[parentID], childID)
	key := fmt.Sprintf("%s:%s", parentID, childID)
	m.ordinals[key] = ordinal
	return nil
}

func (m *mockHierarchyRepository) GetChildChunkIDs(ctx context.Context, parentID uuid.UUID) ([]uuid.UUID, error) {
	children := m.relations[parentID]
	if children == nil {
		return []uuid.UUID{}, nil
	}
	return children, nil
}

func (m *mockHierarchyRepository) GetParentChunkID(ctx context.Context, chunkID uuid.UUID) (mo.Option[uuid.UUID], error) {
	// 逆引き: どの親がこのchunkIDを子として持っているか探す
	for parentID, children := range m.relations {
		for _, childID := range children {
			if childID == chunkID {
				return mo.Some(parentID), nil
			}
		}
	}
	return mo.None[uuid.UUID](), nil // 親が見つからない場合
}

// TestLinkParentChild_Success は正常な親子関係の追加をテストします
func TestLinkParentChild_Success(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()
	builder := NewHierarchyBuilder(repo)

	parentID := uuid.New()
	childID := uuid.New()

	err := builder.LinkParentChild(ctx, parentID, childID, 0)
	require.NoError(t, err)

	// 関係が正しく追加されたことを確認
	children, err := repo.GetChildChunkIDs(ctx, parentID)
	require.NoError(t, err)
	assert.Len(t, children, 1)
	assert.Equal(t, childID, children[0])
}

// TestLinkParentChild_MultipleChildren は複数の子を追加できることをテストします
func TestLinkParentChild_MultipleChildren(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()
	builder := NewHierarchyBuilder(repo)

	parentID := uuid.New()
	child1 := uuid.New()
	child2 := uuid.New()
	child3 := uuid.New()

	err := builder.LinkParentChild(ctx, parentID, child1, 0)
	require.NoError(t, err)

	err = builder.LinkParentChild(ctx, parentID, child2, 1)
	require.NoError(t, err)

	err = builder.LinkParentChild(ctx, parentID, child3, 2)
	require.NoError(t, err)

	// 3つの子が追加されたことを確認
	children, err := repo.GetChildChunkIDs(ctx, parentID)
	require.NoError(t, err)
	assert.Len(t, children, 3)
}

// TestLinkParentChild_InvalidInput は無効な入力に対するエラーをテストします
func TestLinkParentChild_InvalidInput(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()
	builder := NewHierarchyBuilder(repo)

	validID := uuid.New()

	tests := []struct {
		name     string
		parentID uuid.UUID
		childID  uuid.UUID
		ordinal  int
		wantErr  string
	}{
		{
			name:     "nil parent ID",
			parentID: uuid.Nil,
			childID:  validID,
			ordinal:  0,
			wantErr:  "parent ID cannot be nil",
		},
		{
			name:     "nil child ID",
			parentID: validID,
			childID:  uuid.Nil,
			ordinal:  0,
			wantErr:  "child ID cannot be nil",
		},
		{
			name:     "same parent and child",
			parentID: validID,
			childID:  validID,
			ordinal:  0,
			wantErr:  "parent and child cannot be the same chunk",
		},
		{
			name:     "negative ordinal",
			parentID: uuid.New(),
			childID:  uuid.New(),
			ordinal:  -1,
			wantErr:  "ordinal must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := builder.LinkParentChild(ctx, tt.parentID, tt.childID, tt.ordinal)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestValidateTree_NoCircularReference は循環参照がない場合のテストです
func TestValidateTree_NoCircularReference(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()
	builder := NewHierarchyBuilder(repo)

	// ツリー構造を構築: A -> B -> C
	//                        -> D
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()
	d := uuid.New()

	require.NoError(t, builder.LinkParentChild(ctx, a, b, 0))
	require.NoError(t, builder.LinkParentChild(ctx, b, c, 0))
	require.NoError(t, builder.LinkParentChild(ctx, b, d, 1))

	// 循環参照がないことを確認
	err := builder.ValidateTree(ctx, a)
	assert.NoError(t, err)
}

// TestValidateTree_DirectCircularReference は直接的な循環参照(A->B->A)を検出するテストです
func TestValidateTree_DirectCircularReference(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()

	// 循環参照を直接作成: A -> B -> A
	a := uuid.New()
	b := uuid.New()

	// モックに直接関係を追加（バリデーションをバイパス）
	repo.relations[a] = []uuid.UUID{b}
	repo.relations[b] = []uuid.UUID{a}

	builder := NewHierarchyBuilder(repo)

	// 循環参照が検出されることを確認
	err := builder.ValidateTree(ctx, a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular reference detected")
}

// TestValidateTree_IndirectCircularReference は間接的な循環参照(A->B->C->A)を検出するテストです
func TestValidateTree_IndirectCircularReference(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()

	// 循環参照を作成: A -> B -> C -> A
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()

	// モックに直接関係を追加
	repo.relations[a] = []uuid.UUID{b}
	repo.relations[b] = []uuid.UUID{c}
	repo.relations[c] = []uuid.UUID{a}

	builder := NewHierarchyBuilder(repo)

	// 循環参照が検出されることを確認
	err := builder.ValidateTree(ctx, a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular reference detected")
}

// TestValidateTree_ComplexCircularReference は複雑な循環参照を検出するテストです
func TestValidateTree_ComplexCircularReference(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()

	// 複雑な構造: A -> B -> C -> D -> B (BからDへの循環)
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()
	d := uuid.New()

	repo.relations[a] = []uuid.UUID{b}
	repo.relations[b] = []uuid.UUID{c}
	repo.relations[c] = []uuid.UUID{d}
	repo.relations[d] = []uuid.UUID{b}

	builder := NewHierarchyBuilder(repo)

	// 循環参照が検出されることを確認
	err := builder.ValidateTree(ctx, a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular reference detected")
}

// TestValidateTree_MultiLevelHierarchy は多階層の正常なツリーをテストします
func TestValidateTree_MultiLevelHierarchy(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()
	builder := NewHierarchyBuilder(repo)

	// 3階層のツリー構造を構築:
	//        root
	//       /    \
	//      L1A   L1B
	//     /   \
	//   L2A   L2B
	root := uuid.New()
	l1a := uuid.New()
	l1b := uuid.New()
	l2a := uuid.New()
	l2b := uuid.New()

	require.NoError(t, builder.LinkParentChild(ctx, root, l1a, 0))
	require.NoError(t, builder.LinkParentChild(ctx, root, l1b, 1))
	require.NoError(t, builder.LinkParentChild(ctx, l1a, l2a, 0))
	require.NoError(t, builder.LinkParentChild(ctx, l1a, l2b, 1))

	// 循環参照がないことを確認
	err := builder.ValidateTree(ctx, root)
	assert.NoError(t, err)
}

// TestValidateTree_EmptyTree は子がいないノードでもエラーが出ないことをテストします
func TestValidateTree_EmptyTree(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()
	builder := NewHierarchyBuilder(repo)

	root := uuid.New()

	// 子がいない場合でもエラーが出ないことを確認
	err := builder.ValidateTree(ctx, root)
	assert.NoError(t, err)
}

// TestValidateTree_NilRootID はnil rootIDに対するエラーをテストします
func TestValidateTree_NilRootID(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()
	builder := NewHierarchyBuilder(repo)

	err := builder.ValidateTree(ctx, uuid.Nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root ID cannot be nil")
}

// TestGetPathToRoot は指定されたチャンクからルートまでのパスを取得するテストです
func TestGetPathToRoot(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()
	builder := NewHierarchyBuilder(repo)

	// ツリー構造: root -> A -> B -> C
	root := uuid.New()
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()

	require.NoError(t, builder.LinkParentChild(ctx, root, a, 0))
	require.NoError(t, builder.LinkParentChild(ctx, a, b, 0))
	require.NoError(t, builder.LinkParentChild(ctx, b, c, 0))

	// Cからルートまでのパスを取得
	path, err := builder.GetPathToRoot(ctx, c)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{c, b, a, root}, path)

	// ルート自身のパス
	pathRoot, err := builder.GetPathToRoot(ctx, root)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{root}, pathRoot)
}

// TestGetPathToRoot_CircularReference は循環参照時のエラーをテストします
func TestGetPathToRoot_CircularReference(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()

	// 循環参照を作成: A -> B -> A
	a := uuid.New()
	b := uuid.New()

	repo.relations[a] = []uuid.UUID{b}
	repo.relations[b] = []uuid.UUID{a}

	builder := NewHierarchyBuilder(repo)

	// 循環参照が検出されることを確認
	_, err := builder.GetPathToRoot(ctx, b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular reference detected")
}

// TestGetDepth はチャンクの深さを取得するテストです
func TestGetDepth(t *testing.T) {
	ctx := context.Background()
	repo := newMockHierarchyRepository()
	builder := NewHierarchyBuilder(repo)

	// ツリー構造: root -> A -> B -> C
	root := uuid.New()
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()

	require.NoError(t, builder.LinkParentChild(ctx, root, a, 0))
	require.NoError(t, builder.LinkParentChild(ctx, a, b, 0))
	require.NoError(t, builder.LinkParentChild(ctx, b, c, 0))

	// 各ノードの深さを確認
	depthRoot, err := builder.GetDepth(ctx, root)
	require.NoError(t, err)
	assert.Equal(t, 0, depthRoot)

	depthA, err := builder.GetDepth(ctx, a)
	require.NoError(t, err)
	assert.Equal(t, 1, depthA)

	depthB, err := builder.GetDepth(ctx, b)
	require.NoError(t, err)
	assert.Equal(t, 2, depthB)

	depthC, err := builder.GetDepth(ctx, c)
	require.NoError(t, err)
	assert.Equal(t, 3, depthC)
}
