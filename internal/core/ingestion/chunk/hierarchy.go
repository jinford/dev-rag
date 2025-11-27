package chunk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/mo"
)

// HierarchyRepository はチャンク階層関係の読み書きを行うインターフェースです
type HierarchyRepository interface {
	// AddChunkRelation は親子関係を追加します
	AddChunkRelation(ctx context.Context, parentID, childID uuid.UUID, ordinal int) error
	// GetChildChunkIDs は子チャンクのIDリストを取得します
	GetChildChunkIDs(ctx context.Context, parentID uuid.UUID) ([]uuid.UUID, error)
	// GetParentChunkID は親チャンクのIDを取得します（存在しない場合 None）
	GetParentChunkID(ctx context.Context, chunkID uuid.UUID) (mo.Option[uuid.UUID], error)
}

// HierarchyBuilder はチャンクの階層構造を構築・検証するユーティリティです
type HierarchyBuilder struct {
	repo HierarchyRepository
}

// NewHierarchyBuilder は新しいHierarchyBuilderを作成します
func NewHierarchyBuilder(repo HierarchyRepository) *HierarchyBuilder {
	return &HierarchyBuilder{
		repo: repo,
	}
}

// LinkParentChild は親チャンクと子チャンクを関連付けます
// ordinalは子チャンクの順序を表します(0から開始)
func (h *HierarchyBuilder) LinkParentChild(ctx context.Context, parentID, childID uuid.UUID, ordinal int) error {
	if parentID == uuid.Nil {
		return fmt.Errorf("parent ID cannot be nil")
	}
	if childID == uuid.Nil {
		return fmt.Errorf("child ID cannot be nil")
	}
	if parentID == childID {
		return fmt.Errorf("parent and child cannot be the same chunk: %s", parentID)
	}
	if ordinal < 0 {
		return fmt.Errorf("ordinal must be non-negative, got: %d", ordinal)
	}

	// リポジトリ層を通じて親子関係を追加
	if err := h.repo.AddChunkRelation(ctx, parentID, childID, ordinal); err != nil {
		return fmt.Errorf("failed to add chunk relation: %w", err)
	}

	return nil
}

// ValidateTree はrootIDをルートとするチャンク階層ツリーに循環参照がないかチェックします
// 循環参照が検出された場合はエラーを返します
func (h *HierarchyBuilder) ValidateTree(ctx context.Context, rootID uuid.UUID) error {
	if rootID == uuid.Nil {
		return fmt.Errorf("root ID cannot be nil")
	}

	// 訪問済みノードを記録するマップ
	visited := make(map[uuid.UUID]bool)
	// 現在のパス(再帰スタック)を記録するマップ
	recursionStack := make(map[uuid.UUID]bool)

	// DFS(深さ優先探索)で循環参照をチェック
	return h.dfsCheckCycle(ctx, rootID, visited, recursionStack)
}

// dfsCheckCycle は深さ優先探索により循環参照をチェックします
func (h *HierarchyBuilder) dfsCheckCycle(ctx context.Context, nodeID uuid.UUID, visited, recursionStack map[uuid.UUID]bool) error {
	// 現在のノードを訪問済みとしてマーク
	visited[nodeID] = true
	recursionStack[nodeID] = true

	// 子チャンクを取得
	children, err := h.repo.GetChildChunkIDs(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get child chunks for %s: %w", nodeID, err)
	}

	// 各子ノードを再帰的にチェック
	for _, childID := range children {
		// 再帰スタック内に存在する場合は循環参照
		if recursionStack[childID] {
			return fmt.Errorf("circular reference detected: chunk %s is already in the path", childID)
		}

		// 未訪問のノードの場合は再帰的にチェック
		if !visited[childID] {
			if err := h.dfsCheckCycle(ctx, childID, visited, recursionStack); err != nil {
				return err
			}
		}
	}

	// このノードの探索が完了したので再帰スタックから削除
	recursionStack[nodeID] = false

	return nil
}

// GetPathToRoot は指定されたチャンクからルートまでのパスを取得します
// 戻り値は [child, parent, grandparent, ..., root] の順序です
func (h *HierarchyBuilder) GetPathToRoot(ctx context.Context, chunkID uuid.UUID) ([]uuid.UUID, error) {
	if chunkID == uuid.Nil {
		return nil, fmt.Errorf("chunk ID cannot be nil")
	}

	path := []uuid.UUID{chunkID}
	current := chunkID
	visited := make(map[uuid.UUID]bool)

	for {
		// 循環参照チェック
		if visited[current] {
			return nil, fmt.Errorf("circular reference detected while traversing to root from %s", chunkID)
		}
		visited[current] = true

		// 親チャンクを取得
		parentOpt, err := h.repo.GetParentChunkID(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent of %s: %w", current, err)
		}

		// 親がいない場合はルートに到達
		if parentOpt.IsAbsent() {
			break
		}

		parentID := parentOpt.MustGet()
		path = append(path, parentID)
		current = parentID
	}

	return path, nil
}

// GetDepth は指定されたチャンクの深さ(ルートからの距離)を取得します
// ルートチャンクの深さは0です
func (h *HierarchyBuilder) GetDepth(ctx context.Context, chunkID uuid.UUID) (int, error) {
	path, err := h.GetPathToRoot(ctx, chunkID)
	if err != nil {
		return 0, err
	}
	// パスの長さ - 1 が深さ (自分自身も含まれるため)
	return len(path) - 1, nil
}
