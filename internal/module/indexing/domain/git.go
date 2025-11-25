package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// GitHistoryProvider はGit履歴情報を取得するポートです
type GitHistoryProvider interface {
	GetFileEditFrequencies(ctx context.Context, repoPath, ref string, since time.Time) (map[string]*FileEditHistory, error)
}

// DependencyGraphProvider は依存グラフを取得するポートです
type DependencyGraphProvider interface {
	LoadGraphBySnapshot(ctx context.Context, snapshotID uuid.UUID) (DependencyGraph, error)
}
