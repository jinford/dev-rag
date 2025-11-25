package testing

import (
	"time"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// TestProduct はテスト用のProductを生成します
func TestProduct(name string, description *string) *domain.Product {
	return &domain.Product{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// TestSource はテスト用のSourceを生成します
func TestSource(name string, sourceType domain.SourceType, productID uuid.UUID) *domain.Source {
	return &domain.Source{
		ID:         uuid.New(),
		Name:       name,
		SourceType: sourceType,
		ProductID:  productID,
		Metadata:   domain.SourceMetadata{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// TestSourceSnapshot はテスト用のSourceSnapshotを生成します
func TestSourceSnapshot(sourceID uuid.UUID, versionIdentifier string, indexed bool) *domain.SourceSnapshot {
	var indexedAt *time.Time
	if indexed {
		now := time.Now()
		indexedAt = &now
	}

	return &domain.SourceSnapshot{
		ID:                uuid.New(),
		SourceID:          sourceID,
		VersionIdentifier: versionIdentifier,
		IndexedAt:         indexedAt,
		CreatedAt:         time.Now(),
	}
}

// TestFile はテスト用のFileを生成します
func TestFile(snapshotID uuid.UUID, path string, language *string) *domain.File {
	return &domain.File{
		ID:          uuid.New(),
		SnapshotID:  snapshotID,
		Path:        path,
		Size:        1024,
		ContentType: "text/plain",
		ContentHash: "hash123",
		Language:    language,
		CreatedAt:   time.Now(),
	}
}

// TestChunk はテスト用のChunkを生成します
func TestChunk(fileID uuid.UUID, ordinal int, content string) *domain.Chunk {
	return &domain.Chunk{
		ID:          uuid.New(),
		FileID:      fileID,
		Ordinal:     ordinal,
		StartLine:   1,
		EndLine:     10,
		Content:     content,
		ContentHash: "hash123",
		TokenCount:  100,
		CreatedAt:   time.Now(),
	}
}
