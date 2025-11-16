package models

import "time"

// Job はジョブの状況を表します
type Job struct {
	JobID      string     `json:"jobID"`
	TargetType string     `json:"targetType"` // "product", "source"
	TargetName string     `json:"targetName"`
	JobType    string     `json:"jobType"` // "index", "wiki"
	Status     string     `json:"status"`  // "running", "completed", "failed"
	StartedAt  time.Time  `json:"startedAt"`
	EndedAt    *time.Time `json:"endedAt,omitempty"`
	Error      string     `json:"error,omitempty"`
}
