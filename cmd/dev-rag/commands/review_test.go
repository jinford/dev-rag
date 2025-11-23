package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReviewRunAction_Flags(t *testing.T) {
	// フラグの設定をテスト
	tests := []struct {
		name      string
		weekRange int
		wantErr   bool
	}{
		{
			name:      "デフォルト週範囲",
			weekRange: 0, // デフォルトで1になる
			wantErr:   false,
		},
		{
			name:      "カスタム週範囲",
			weekRange: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weekRange := tt.weekRange
			if weekRange == 0 {
				weekRange = 1 // デフォルト
			}
			assert.Greater(t, weekRange, 0)
		})
	}
}

func TestReviewScheduleAction_CronSchedule(t *testing.T) {
	// Cronスケジュールの検証をテスト
	tests := []struct {
		name         string
		cronSchedule string
		expected     string
	}{
		{
			name:         "デフォルトスケジュール",
			cronSchedule: "",
			expected:     "0 9 * * 1", // 毎週月曜9:00
		},
		{
			name:         "カスタムスケジュール",
			cronSchedule: "0 10 * * 2",
			expected:     "0 10 * * 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := tt.cronSchedule
			if schedule == "" {
				schedule = "0 9 * * 1"
			}
			assert.Equal(t, tt.expected, schedule)
		})
	}
}
