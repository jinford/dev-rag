package application_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/application"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
	testutil "github.com/jinford/dev-rag/internal/module/indexing/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceService_GetSource_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	sourceID := uuid.New()
	expectedSource := testutil.TestSource("test-source", domain.SourceTypeGit, uuid.New())
	expectedSource.ID = sourceID

	mockRepo := &testutil.MockSourceReader{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Source, error) {
			assert.Equal(t, sourceID, id)
			return expectedSource, nil
		},
	}

	service := application.NewSourceService(mockRepo, log)

	// Execute
	result, err := service.GetSource(ctx, sourceID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedSource, result)
}

func TestSourceService_GetSource_NilID(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockRepo := &testutil.MockSourceReader{}
	service := application.NewSourceService(mockRepo, log)

	// Execute
	result, err := service.GetSource(ctx, uuid.Nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "source ID is required")
}

func TestSourceService_GetSource_RepositoryError(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	sourceID := uuid.New()
	expectedErr := errors.New("database error")

	mockRepo := &testutil.MockSourceReader{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Source, error) {
			return nil, expectedErr
		},
	}

	service := application.NewSourceService(mockRepo, log)

	// Execute
	result, err := service.GetSource(ctx, sourceID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get source")
	assert.ErrorIs(t, err, expectedErr)
}

func TestSourceService_GetSourceByName_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	sourceName := "test-source"
	expectedSource := testutil.TestSource(sourceName, domain.SourceTypeGit, uuid.New())

	mockRepo := &testutil.MockSourceReader{
		GetByNameFunc: func(ctx context.Context, name string) (*domain.Source, error) {
			assert.Equal(t, sourceName, name)
			return expectedSource, nil
		},
	}

	service := application.NewSourceService(mockRepo, log)

	// Execute
	result, err := service.GetSourceByName(ctx, sourceName)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedSource, result)
}

func TestSourceService_GetSourceByName_EmptyName(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockRepo := &testutil.MockSourceReader{}
	service := application.NewSourceService(mockRepo, log)

	// Execute
	result, err := service.GetSourceByName(ctx, "")

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "source name is required")
}

func TestSourceService_ListSources_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	productID := uuid.New()
	expectedSources := []*domain.Source{
		testutil.TestSource("source1", domain.SourceTypeGit, productID),
		testutil.TestSource("source2", domain.SourceTypeLocal, productID),
	}

	mockRepo := &testutil.MockSourceReader{
		ListByProductIDFunc: func(ctx context.Context, pid uuid.UUID) ([]*domain.Source, error) {
			assert.Equal(t, productID, pid)
			return expectedSources, nil
		},
	}

	service := application.NewSourceService(mockRepo, log)

	// Execute
	filter := application.SourceFilter{ProductID: &productID}
	result, err := service.ListSources(ctx, filter)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedSources, result)
	assert.Len(t, result, 2)
}

func TestSourceService_ListSources_NoFilter(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockRepo := &testutil.MockSourceReader{}
	service := application.NewSourceService(mockRepo, log)

	// Execute
	filter := application.SourceFilter{}
	result, err := service.ListSources(ctx, filter)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "product ID filter is required")
}

func TestSourceService_GetLatestSnapshot_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	sourceID := uuid.New()
	expectedSnapshot := testutil.TestSourceSnapshot(sourceID, "v1.0.0", true)

	mockRepo := &testutil.MockSourceReader{
		GetLatestIndexedSnapshotFunc: func(ctx context.Context, sid uuid.UUID) (*domain.SourceSnapshot, error) {
			assert.Equal(t, sourceID, sid)
			return expectedSnapshot, nil
		},
	}

	service := application.NewSourceService(mockRepo, log)

	// Execute
	result, err := service.GetLatestSnapshot(ctx, sourceID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedSnapshot, result)
}

func TestSourceService_GetLatestSnapshot_NilID(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockRepo := &testutil.MockSourceReader{}
	service := application.NewSourceService(mockRepo, log)

	// Execute
	result, err := service.GetLatestSnapshot(ctx, uuid.Nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "source ID is required")
}

func TestSourceService_ListSnapshots_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	sourceID := uuid.New()
	expectedSnapshots := []*domain.SourceSnapshot{
		testutil.TestSourceSnapshot(sourceID, "v1.0.0", true),
		testutil.TestSourceSnapshot(sourceID, "v1.1.0", true),
		testutil.TestSourceSnapshot(sourceID, "v1.2.0", false),
	}

	mockRepo := &testutil.MockSourceReader{
		ListSnapshotsBySourceFunc: func(ctx context.Context, sid uuid.UUID) ([]*domain.SourceSnapshot, error) {
			assert.Equal(t, sourceID, sid)
			return expectedSnapshots, nil
		},
	}

	service := application.NewSourceService(mockRepo, log)

	// Execute
	result, err := service.ListSnapshots(ctx, sourceID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedSnapshots, result)
	assert.Len(t, result, 3)
}

func TestSourceService_ListSnapshots_NilID(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockRepo := &testutil.MockSourceReader{}
	service := application.NewSourceService(mockRepo, log)

	// Execute
	result, err := service.ListSnapshots(ctx, uuid.Nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "source ID is required")
}

func TestSourceService_ListSnapshots_RepositoryError(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	sourceID := uuid.New()
	expectedErr := errors.New("database error")

	mockRepo := &testutil.MockSourceReader{
		ListSnapshotsBySourceFunc: func(ctx context.Context, sid uuid.UUID) ([]*domain.SourceSnapshot, error) {
			return nil, expectedErr
		},
	}

	service := application.NewSourceService(mockRepo, log)

	// Execute
	result, err := service.ListSnapshots(ctx, sourceID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list snapshots")
	assert.ErrorIs(t, err, expectedErr)
}
