package domain

import "errors"

var (
	// ErrRateLimitExceeded はレート制限を超えた場合のエラー
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrInvalidRequest はリクエストが不正な場合のエラー
	ErrInvalidRequest = errors.New("invalid request")

	// ErrModelNotAvailable はモデルが利用できない場合のエラー
	ErrModelNotAvailable = errors.New("model not available")

	// ErrContextCanceled はコンテキストがキャンセルされた場合のエラー
	ErrContextCanceled = errors.New("context canceled")

	// ErrMaxRetriesExceeded は最大リトライ回数を超えた場合のエラー
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)
