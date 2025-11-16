package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const pgErrCodeUniqueViolation = "23505"

// IsUniqueViolation は PostgreSQL の unique_violation(23505) かどうかを判定します
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgErrCodeUniqueViolation
	}
	return false
}
