package models

import "time"

// Job は非同期ジョブの状態を表します（フェーズ5: HTTPサーバで使用）
type Job struct {
	ID         string    `json:"id"`
	TargetType string    `json:"targetType"` // "product" or "source"
	TargetName string    `json:"targetName"`
	JobType    string    `json:"jobType"` // "index" or "wiki"
	Status     string    `json:"status"`  // "running", "completed", "failed"
	ErrorMsg   *string   `json:"errorMsg,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
