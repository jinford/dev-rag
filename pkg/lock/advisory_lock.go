package lock

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AdvisoryLock はPostgreSQLのアドバイザリロックを管理します
type AdvisoryLock struct {
	tx     pgx.Tx
	lockID int64
}

// Manager はアドバイザリロックの取得を仲介します
type Manager struct {
	tx pgx.Tx
}

// NewManager はトランザクションからロックマネージャーを生成します
func NewManager(tx pgx.Tx) *Manager {
	return &Manager{tx: tx}
}

// GenerateLockID は文字列からロックIDを生成します
func GenerateLockID(parts ...string) int64 {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
	}
	hash := h.Sum(nil)

	// ハッシュの最初の8バイトをint64として使用
	var id int64
	for i := range 8 {
		id = (id << 8) | int64(hash[i])
	}

	return id
}

// Acquire はPostgreSQLアドバイザリロックを取得します
// トランザクションスコープのロックを使用（pg_advisory_xact_lock）
func Acquire(ctx context.Context, tx pgx.Tx, lockID int64) (*AdvisoryLock, error) {
	return acquire(ctx, tx, lockID)
}

// Acquire はManager経由でも呼び出せます
func (m *Manager) Acquire(ctx context.Context, lockID int64) (*AdvisoryLock, error) {
	return acquire(ctx, m.tx, lockID)
}

func acquire(ctx context.Context, tx pgx.Tx, lockID int64) (*AdvisoryLock, error) {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockID)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire advisory lock: %w", err)
	}

	return &AdvisoryLock{
		tx:     tx,
		lockID: lockID,
	}, nil
}

// Release はアドバイザリロックを解放します
// 注: トランザクションスコープのロック（pg_advisory_xact_lock）を使用しているため、
// トランザクション終了時に自動的に解放されます。このメソッドは明示的な解放が不要です。
func (l *AdvisoryLock) Release(ctx context.Context) error {
	// トランザクションスコープのロックは自動解放されるため、何もしない
	return nil
}
