package domain

import (
	"context"

	"github.com/google/uuid"
)

// RepositoryAnalyzer はリポジトリ全体を解析してサマリーを生成するインターフェース
type RepositoryAnalyzer interface {
	// AnalyzeRepository はリポジトリ全体を解析し、
	// リポジトリ構造を返します
	AnalyzeRepository(ctx context.Context, sourceID, snapshotID uuid.UUID) (*RepoStructure, error)
}
