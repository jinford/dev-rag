package application_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jinford/dev-rag/internal/module/indexing/application"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	testutil "github.com/jinford/dev-rag/internal/module/indexing/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexService_IndexSource_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockIndexer := &testutil.MockIndexer{
		IndexSourceFunc: func(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*application.IndexResult, error) {
			return &application.IndexResult{
				SnapshotID:        "snapshot-123",
				VersionIdentifier: "v1.0.0",
				ProcessedFiles:    10,
				TotalChunks:       50,
				Duration:          1 * time.Minute,
			}, nil
		},
	}

	service := application.NewIndexService(mockIndexer, log)

	// Execute
	params := domain.IndexParams{
		Identifier:  "test-repo",
		ProductName: "test-product",
		ForceInit:   false,
	}
	result, err := service.IndexSource(ctx, domain.SourceTypeGit, params)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "snapshot-123", result.SnapshotID)
	assert.Equal(t, "v1.0.0", result.VersionIdentifier)
	assert.Equal(t, 10, result.ProcessedFiles)
	assert.Equal(t, 50, result.TotalChunks)
	assert.Equal(t, 1*time.Minute, result.Duration)
}

func TestIndexService_IndexSource_MissingIdentifier(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockIndexer := &testutil.MockIndexer{}
	service := application.NewIndexService(mockIndexer, log)

	// Execute
	params := domain.IndexParams{
		Identifier:  "", // Missing identifier
		ProductName: "test-product",
	}
	result, err := service.IndexSource(ctx, domain.SourceTypeGit, params)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "identifier is required")
}

func TestIndexService_IndexSource_MissingProductName(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockIndexer := &testutil.MockIndexer{}
	service := application.NewIndexService(mockIndexer, log)

	// Execute
	params := domain.IndexParams{
		Identifier:  "test-repo",
		ProductName: "", // Missing product name
	}
	result, err := service.IndexSource(ctx, domain.SourceTypeGit, params)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "product name is required")
}

func TestIndexService_IndexSource_IndexerError(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	expectedErr := errors.New("indexer failed")
	mockIndexer := &testutil.MockIndexer{
		IndexSourceFunc: func(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*application.IndexResult, error) {
			return nil, expectedErr
		},
	}

	service := application.NewIndexService(mockIndexer, log)

	// Execute
	params := domain.IndexParams{
		Identifier:  "test-repo",
		ProductName: "test-product",
	}
	result, err := service.IndexSource(ctx, domain.SourceTypeGit, params)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to index source")
	assert.ErrorIs(t, err, expectedErr)
}

func TestIndexService_ReindexSource(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	var capturedParams domain.IndexParams
	mockIndexer := &testutil.MockIndexer{
		IndexSourceFunc: func(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*application.IndexResult, error) {
			capturedParams = params
			return &application.IndexResult{
				SnapshotID:        "snapshot-456",
				VersionIdentifier: "v2.0.0",
				ProcessedFiles:    15,
				TotalChunks:       75,
				Duration:          2 * time.Minute,
			}, nil
		},
	}

	service := application.NewIndexService(mockIndexer, log)

	// Execute
	params := domain.IndexParams{
		Identifier:  "test-repo",
		ProductName: "test-product",
		ForceInit:   false, // Should be set to true by ReindexSource
	}
	result, err := service.ReindexSource(ctx, domain.SourceTypeGit, params)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, capturedParams.ForceInit, "ForceInit should be set to true for reindex")
	assert.Equal(t, "snapshot-456", result.SnapshotID)
	assert.Equal(t, "v2.0.0", result.VersionIdentifier)
}

func TestIndexService_IndexSource_DifferentSourceTypes(t *testing.T) {
	tests := []struct {
		name       string
		sourceType domain.SourceType
	}{
		{
			name:       "Git source",
			sourceType: domain.SourceTypeGit,
		},
		{
			name:       "Local source",
			sourceType: domain.SourceTypeLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ctx := context.Background()
			log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

			var capturedSourceType domain.SourceType
			mockIndexer := &testutil.MockIndexer{
				IndexSourceFunc: func(ctx context.Context, sourceType domain.SourceType, params domain.IndexParams) (*application.IndexResult, error) {
					capturedSourceType = sourceType
					return &application.IndexResult{
						SnapshotID:        "snapshot-789",
						VersionIdentifier: "v3.0.0",
						ProcessedFiles:    5,
						TotalChunks:       25,
						Duration:          30 * time.Second,
					}, nil
				},
			}

			service := application.NewIndexService(mockIndexer, log)

			// Execute
			params := domain.IndexParams{
				Identifier:  "test-repo",
				ProductName: "test-product",
			}
			result, err := service.IndexSource(ctx, tt.sourceType, params)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.sourceType, capturedSourceType)
		})
	}
}
