package dependency

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	domaindep "github.com/jinford/dev-rag/internal/module/indexing/domain/dependency"
)

// GraphProvider は domain.DependencyGraphProvider の実装です
type GraphProvider struct {
	chunkReader      domain.ChunkReader
	fileReader       domain.FileReader
	dependencyReader domain.DependencyReader
}

// NewGraphProvider は新しいGraphProviderを作成します
func NewGraphProvider(
	chunkReader domain.ChunkReader,
	fileReader domain.FileReader,
	dependencyReader domain.DependencyReader,
) *GraphProvider {
	return &GraphProvider{
		chunkReader:      chunkReader,
		fileReader:       fileReader,
		dependencyReader: dependencyReader,
	}
}

// LoadGraphBySnapshot はスナップショットIDから依存グラフを読み込みます
func (p *GraphProvider) LoadGraphBySnapshot(ctx context.Context, snapshotID uuid.UUID) (domain.DependencyGraph, error) {
	// 1. スナップショットに属するファイルを取得
	files, err := p.fileReader.ListBySnapshot(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// 2. 各ファイルのチャンクを取得し、グラフを構築
	graph := domaindep.NewGraph()
	filePathMap := make(map[uuid.UUID]string) // ChunkID -> FilePath のマッピング

	for _, file := range files {
		chunks, err := p.chunkReader.ListByFile(ctx, file.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list chunks for file %s: %w", file.ID, err)
		}

		// チャンクをグラフのノードとして追加
		for _, chunk := range chunks {
			node := &domaindep.Node{
				ChunkID:  chunk.ID,
				FilePath: file.Path,
			}

			// チャンクからノード情報を設定
			if chunk.Name != nil {
				node.Name = *chunk.Name
			}
			if chunk.Type != nil {
				node.Type = *chunk.Type
			}

			graph.AddNode(node)
			filePathMap[chunk.ID] = file.Path
		}

		// 依存関係をエッジとして追加
		for _, chunk := range chunks {
			deps, err := p.dependencyReader.GetByChunk(ctx, chunk.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get dependencies for chunk %s: %w", chunk.ID, err)
			}

			for _, dep := range deps {
				edge := &domaindep.Edge{
					From: dep.FromChunkID,
					To:   dep.ToChunkID,
				}

				// 依存タイプを変換
				switch dep.DepType {
				case "call":
					edge.RelationType = domaindep.RelationTypeCalls
				case "import":
					edge.RelationType = domaindep.RelationTypeImports
				case "type":
					edge.RelationType = domaindep.RelationTypeUses
				default:
					edge.RelationType = domaindep.RelationTypeUnknown
				}

				edge.Weight = 1 // デフォルト重み

				// エッジを追加（エラーは無視 - ノードが存在しない場合がある）
				_ = graph.AddEdge(edge)
			}
		}
	}

	return graph, nil
}
