package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SnapshotService はスナップショットの生成と状態管理を行うサービスインターフェース
type SnapshotService interface {
	// CreateSnapshot は新しいスナップショットを作成し、IDを発行します
	CreateSnapshot(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*SourceSnapshot, error)

	// GetOrCreateSnapshot は既存のスナップショットを取得するか、なければ新規作成します
	GetOrCreateSnapshot(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*SourceSnapshot, error)

	// MarkAsIndexed はスナップショットをインデックス済みとしてマークします
	MarkAsIndexed(ctx context.Context, snapshotID uuid.UUID, indexedAt time.Time) error

	// GetLatestSnapshot は指定されたソースの最新スナップショットを取得します
	GetLatestSnapshot(ctx context.Context, sourceID uuid.UUID) (*SourceSnapshot, error)
}

// SnapshotRepository はスナップショットの永続化を担当するリポジトリインターフェース
// ※ repository.go に定義済みの場合はそちらを優先し、このファイルは補完的な定義として使用
type SnapshotRepository interface {
	// Create は新しいスナップショットを永続化します
	Create(ctx context.Context, snapshot *SourceSnapshot) error

	// FindByID はIDでスナップショットを取得します
	FindByID(ctx context.Context, id uuid.UUID) (*SourceSnapshot, error)

	// FindBySourceAndVersion はソースIDとバージョン識別子でスナップショットを取得します
	FindBySourceAndVersion(ctx context.Context, sourceID uuid.UUID, versionIdentifier string) (*SourceSnapshot, error)

	// UpdateIndexStatus はスナップショットのインデックス状態を更新します
	UpdateIndexStatus(ctx context.Context, snapshotID uuid.UUID, indexed bool, indexedAt *time.Time) error

	// GetLatestBySource は指定されたソースの最新スナップショットを取得します
	GetLatestBySource(ctx context.Context, sourceID uuid.UUID) (*SourceSnapshot, error)
}
