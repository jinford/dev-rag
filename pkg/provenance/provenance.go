package provenance

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ChunkProvenance はチャンクのデータ起源情報を表します
type ChunkProvenance struct {
	ChunkID          uuid.UUID  `json:"chunkID"`
	SnapshotID       uuid.UUID  `json:"snapshotID"`
	FilePath         string     `json:"filePath"`
	GitCommitHash    string     `json:"gitCommitHash"`
	ChunkKey         string     `json:"chunkKey"` // {product_name}/{source_name}/{file_path}#L{start}-L{end}@{commit_hash}
	IsLatest         bool       `json:"isLatest"`
	Author           *string    `json:"author,omitempty"`
	UpdatedAt        *time.Time `json:"updatedAt,omitempty"`
	IndexedAt        time.Time  `json:"indexedAt"`
	FileVersion      *string    `json:"fileVersion,omitempty"`
	SourceSnapshotID uuid.UUID  `json:"sourceSnapshotID"`
}

// FileProvenanceHistory は同一ファイルの異なるバージョンのチャンク履歴を表します
type FileProvenanceHistory struct {
	FilePath string              `json:"filePath"`
	Versions []*ChunkProvenance  `json:"versions"`
}

// ProvenanceGraph はチャンクIDから起源情報へのマッピングを管理します
// Phase 2では軽量なインメモリ実装とし、Phase 3以降でデータベース永続化を検討
type ProvenanceGraph struct {
	mu sync.RWMutex
	// チャンクID -> Provenance情報
	provenances map[uuid.UUID]*ChunkProvenance
	// ChunkKey -> ChunkID のマッピング (同一ファイル・同一範囲の異なるバージョンを追跡)
	keyToChunks map[string][]uuid.UUID
	// FilePath -> ChunkID のマッピング (ファイル別の履歴追跡)
	fileToChunks map[string][]uuid.UUID
}

// NewProvenanceGraph は新しいProvenanceGraphを生成します
func NewProvenanceGraph() *ProvenanceGraph {
	return &ProvenanceGraph{
		provenances:  make(map[uuid.UUID]*ChunkProvenance),
		keyToChunks:  make(map[string][]uuid.UUID),
		fileToChunks: make(map[string][]uuid.UUID),
	}
}

// Add はチャンクのProvenance情報をグラフに追加します
func (pg *ProvenanceGraph) Add(prov *ChunkProvenance) error {
	if prov == nil {
		return fmt.Errorf("provenance cannot be nil")
	}
	if prov.ChunkID == uuid.Nil {
		return fmt.Errorf("chunk ID cannot be nil")
	}
	if prov.SnapshotID == uuid.Nil {
		return fmt.Errorf("snapshot ID cannot be nil")
	}
	if prov.FilePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	pg.mu.Lock()
	defer pg.mu.Unlock()

	// チャンクIDのマッピングを追加
	pg.provenances[prov.ChunkID] = prov

	// ChunkKeyベースのマッピングを追加
	if prov.ChunkKey != "" {
		pg.keyToChunks[prov.ChunkKey] = append(pg.keyToChunks[prov.ChunkKey], prov.ChunkID)
	}

	// FilePathベースのマッピングを追加
	pg.fileToChunks[prov.FilePath] = append(pg.fileToChunks[prov.FilePath], prov.ChunkID)

	return nil
}

// Get はチャンクIDからProvenance情報を取得します
func (pg *ProvenanceGraph) Get(chunkID uuid.UUID) (*ChunkProvenance, error) {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	prov, ok := pg.provenances[chunkID]
	if !ok {
		return nil, fmt.Errorf("provenance not found for chunk ID: %s", chunkID)
	}

	return prov, nil
}

// GetByChunkKey はChunkKeyから全てのバージョンのチャンクIDを取得します
// 同一ファイル・同一範囲の異なるコミットハッシュのチャンクを返します
func (pg *ProvenanceGraph) GetByChunkKey(chunkKey string) ([]uuid.UUID, error) {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	chunkIDs, ok := pg.keyToChunks[chunkKey]
	if !ok || len(chunkIDs) == 0 {
		return nil, fmt.Errorf("no chunks found for chunk key: %s", chunkKey)
	}

	return chunkIDs, nil
}

// GetLatestByChunkKey はChunkKeyから最新バージョンのチャンクIDを取得します
func (pg *ProvenanceGraph) GetLatestByChunkKey(chunkKey string) (*ChunkProvenance, error) {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	chunkIDs, ok := pg.keyToChunks[chunkKey]
	if !ok || len(chunkIDs) == 0 {
		return nil, fmt.Errorf("no chunks found for chunk key: %s", chunkKey)
	}

	// IsLatestフラグが立っているものを探す
	for _, chunkID := range chunkIDs {
		prov, ok := pg.provenances[chunkID]
		if ok && prov.IsLatest {
			return prov, nil
		}
	}

	// IsLatestフラグがない場合は、IndexedAtが最新のものを返す
	var latest *ChunkProvenance
	for _, chunkID := range chunkIDs {
		prov, ok := pg.provenances[chunkID]
		if !ok {
			continue
		}
		if latest == nil || prov.IndexedAt.After(latest.IndexedAt) {
			latest = prov
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no valid provenance found for chunk key: %s", chunkKey)
	}

	return latest, nil
}

// GetFileHistory はファイルパスから全てのバージョンのチャンク履歴を取得します
func (pg *ProvenanceGraph) GetFileHistory(filePath string) (*FileProvenanceHistory, error) {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	chunkIDs, ok := pg.fileToChunks[filePath]
	if !ok || len(chunkIDs) == 0 {
		return nil, fmt.Errorf("no chunks found for file path: %s", filePath)
	}

	history := &FileProvenanceHistory{
		FilePath: filePath,
		Versions: make([]*ChunkProvenance, 0, len(chunkIDs)),
	}

	for _, chunkID := range chunkIDs {
		prov, ok := pg.provenances[chunkID]
		if ok {
			history.Versions = append(history.Versions, prov)
		}
	}

	return history, nil
}

// GetLatestVersions は全てのチャンクの中から最新バージョンのみを取得します
func (pg *ProvenanceGraph) GetLatestVersions() []*ChunkProvenance {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	latest := make([]*ChunkProvenance, 0)
	for _, prov := range pg.provenances {
		if prov.IsLatest {
			latest = append(latest, prov)
		}
	}

	return latest
}

// Count は登録されているProvenance情報の総数を返します
func (pg *ProvenanceGraph) Count() int {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	return len(pg.provenances)
}

// IsLatest はチャンクIDが最新バージョンかどうかを判定します
func (pg *ProvenanceGraph) IsLatest(chunkID uuid.UUID) (bool, error) {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	prov, ok := pg.provenances[chunkID]
	if !ok {
		return false, fmt.Errorf("provenance not found for chunk ID: %s", chunkID)
	}

	return prov.IsLatest, nil
}

// TraceProvenance はチャンクIDから起源情報を追跡し、人間が読める形式で返します
func (pg *ProvenanceGraph) TraceProvenance(chunkID uuid.UUID) (string, error) {
	prov, err := pg.Get(chunkID)
	if err != nil {
		return "", err
	}

	trace := fmt.Sprintf("Chunk ID: %s\n", prov.ChunkID)
	trace += fmt.Sprintf("File: %s\n", prov.FilePath)
	trace += fmt.Sprintf("Git Commit: %s\n", prov.GitCommitHash)
	trace += fmt.Sprintf("Snapshot ID: %s\n", prov.SnapshotID)
	trace += fmt.Sprintf("Source Snapshot ID: %s\n", prov.SourceSnapshotID)
	trace += fmt.Sprintf("Chunk Key: %s\n", prov.ChunkKey)
	trace += fmt.Sprintf("Is Latest: %v\n", prov.IsLatest)
	trace += fmt.Sprintf("Indexed At: %s\n", prov.IndexedAt.Format(time.RFC3339))

	if prov.Author != nil {
		trace += fmt.Sprintf("Author: %s\n", *prov.Author)
	}
	if prov.UpdatedAt != nil {
		trace += fmt.Sprintf("Updated At: %s\n", prov.UpdatedAt.Format(time.RFC3339))
	}

	return trace, nil
}

// Clear は全てのProvenance情報をクリアします（テスト用）
func (pg *ProvenanceGraph) Clear() {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	pg.provenances = make(map[uuid.UUID]*ChunkProvenance)
	pg.keyToChunks = make(map[string][]uuid.UUID)
	pg.fileToChunks = make(map[string][]uuid.UUID)
}
