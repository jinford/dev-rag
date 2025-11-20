package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDomainCoverageStruct はDomainCoverage構造体が正しく定義されていることを確認します
func TestDomainCoverageStruct(t *testing.T) {
	coverage := &DomainCoverage{
		Domain:     "code",
		FileCount:  10,
		ChunkCount: 50,
	}

	assert.Equal(t, "code", coverage.Domain)
	assert.Equal(t, int64(10), coverage.FileCount)
	assert.Equal(t, int64(50), coverage.ChunkCount)
}

// TestDomainCoverageFields はすべてのフィールドが設定可能であることを確認します
func TestDomainCoverageFields(t *testing.T) {
	tests := []struct {
		name       string
		domain     string
		fileCount  int64
		chunkCount int64
	}{
		{
			name:       "code domain",
			domain:     "code",
			fileCount:  5,
			chunkCount: 25,
		},
		{
			name:       "tests domain",
			domain:     "tests",
			fileCount:  3,
			chunkCount: 15,
		},
		{
			name:       "architecture domain",
			domain:     "architecture",
			fileCount:  2,
			chunkCount: 10,
		},
		{
			name:       "ops domain",
			domain:     "ops",
			fileCount:  1,
			chunkCount: 5,
		},
		{
			name:       "infra domain",
			domain:     "infra",
			fileCount:  4,
			chunkCount: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coverage := &DomainCoverage{
				Domain:     tt.domain,
				FileCount:  tt.fileCount,
				ChunkCount: tt.chunkCount,
			}

			assert.Equal(t, tt.domain, coverage.Domain)
			assert.Equal(t, tt.fileCount, coverage.FileCount)
			assert.Equal(t, tt.chunkCount, coverage.ChunkCount)
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
