package domain

import (
	"context"
)

// Lock はアドバイザリロックを表すインターフェース
type Lock interface {
	// Release はロックを解放します
	Release(ctx context.Context) error
}

// LockManager はアドバイザリロックの取得を管理するインターフェース
type LockManager interface {
	// Acquire はロックを取得します
	// lockID はロックを識別する一意のID
	Acquire(ctx context.Context, lockID int64) (Lock, error)
}
