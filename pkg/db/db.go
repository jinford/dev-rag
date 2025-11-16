package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB はデータベース接続プールを保持します
type DB struct {
	Pool *pgxpool.Pool
}

// ConnectionParams はデータベース接続パラメータ
type ConnectionParams struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// New は新しいデータベース接続を作成します
func New(ctx context.Context, params ConnectionParams) (*DB, error) {
	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		params.Host,
		params.Port,
		params.User,
		params.Password,
		params.DBName,
		params.SSLMode,
	)
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// 接続テスト
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close はデータベース接続を閉じます
func (db *DB) Close() {
	db.Pool.Close()
}
