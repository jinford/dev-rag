package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jinford/dev-rag/pkg/models"
	"github.com/jinford/dev-rag/pkg/sqlc"
	pgvector "github.com/pgvector/pgvector-go"
)

// IndexRepositoryR はファイル/チャンク/Embedding 集約への読み取り専用アクセスを提供します
type IndexRepositoryR struct {
	q sqlc.Querier
}

// NewIndexRepositoryR は sqlc の DBTX を受け取り、読み取り専用リポジトリを初期化します
func NewIndexRepositoryR(q sqlc.Querier) *IndexRepositoryR {
	return &IndexRepositoryR{q: q}
}

// IndexRepositoryRW は IndexRepositoryR を埋め込み、読み書き操作を提供します
type IndexRepositoryRW struct {
	*IndexRepositoryR
}

// NewIndexRepositoryRW は読み書き操作を提供するリポジトリを初期化します
func NewIndexRepositoryRW(q sqlc.Querier) *IndexRepositoryRW {
	return &IndexRepositoryRW{IndexRepositoryR: NewIndexRepositoryR(q)}
}

// SearchFilter は検索時の任意フィルタを表します
type SearchFilter struct {
	PathPrefix  *string
	ContentType *string
}

// ChunkMetadata はチャンクのメタデータを表します
type ChunkMetadata struct {
	// 構造メタデータ
	Type                 *string
	Name                 *string
	ParentName           *string
	Signature            *string
	DocComment           *string
	Imports              []string
	Calls                []string
	LinesOfCode          *int
	CommentRatio         *float64
	CyclomaticComplexity *int
	EmbeddingContext     *string

	// 階層関係と重要度
	Level           int
	ImportanceScore *float64

	// 詳細な依存関係情報
	StandardImports  []string `json:"standardImports,omitempty"`  // 標準ライブラリ
	ExternalImports  []string `json:"externalImports,omitempty"`  // 外部依存
	InternalCalls    []string `json:"internalCalls,omitempty"`    // 内部関数呼び出し
	ExternalCalls    []string `json:"externalCalls,omitempty"`    // 外部関数呼び出し
	TypeDependencies []string `json:"typeDependencies,omitempty"` // 型依存

	// トレーサビリティ・バージョン管理
	SourceSnapshotID *uuid.UUID
	GitCommitHash    *string
	Author           *string
	UpdatedAt        *time.Time // ファイル最終更新日時
	FileVersion      *string
	IsLatest         bool

	// 決定的な識別子
	ChunkKey string // {product_name}/{source_name}/{file_path}#L{start}-L{end}@{commit_hash}
}

// === File 操作 ===

// CreateFile はファイルレコードを作成します
func (rw *IndexRepositoryRW) CreateFile(ctx context.Context, snapshotID uuid.UUID, path string, size int64, contentType string, contentHash string, language *string, domain *string) (*models.File, error) {
	file, err := rw.q.CreateFile(ctx, sqlc.CreateFileParams{
		SnapshotID:  UUIDToPgtype(snapshotID),
		Path:        path,
		Size:        size,
		ContentType: contentType,
		ContentHash: contentHash,
		Language:    StringPtrToPgtext(language),
		Domain:      StringPtrToPgtext(domain),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return convertSQLCFile(file), nil
}

// GetFileByID は ID でファイルを取得します
func (r *IndexRepositoryR) GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error) {
	file, err := r.q.GetFile(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("file not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return convertSQLCFile(file), nil
}

// ListFilesBySnapshot はスナップショット配下のファイル一覧を取得します
func (r *IndexRepositoryR) ListFilesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*models.File, error) {
	rows, err := r.q.ListFilesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]*models.File, 0, len(rows))
	for _, row := range rows {
		files = append(files, convertSQLCFile(row))
	}

	return files, nil
}

// GetFileHashesBySnapshot は差分判定用に path->hash を返します
func (r *IndexRepositoryR) GetFileHashesBySnapshot(ctx context.Context, snapshotID uuid.UUID) (map[string]string, error) {
	rows, err := r.q.GetFileHashesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to get file hashes: %w", err)
	}

	hashes := make(map[string]string, len(rows))
	for _, row := range rows {
		hashes[row.Path] = row.ContentHash
	}

	return hashes, nil
}

// DeleteFileByID は単一ファイルを削除します
func (rw *IndexRepositoryRW) DeleteFileByID(ctx context.Context, id uuid.UUID) error {
	if _, err := rw.q.GetFile(ctx, UUIDToPgtype(id)); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("file not found: %s", id)
		}
		return fmt.Errorf("failed to get file: %w", err)
	}

	if err := rw.q.DeleteFile(ctx, UUIDToPgtype(id)); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// DeleteFilesByPaths は snapshot+paths 条件で一括削除します
func (rw *IndexRepositoryRW) DeleteFilesByPaths(ctx context.Context, snapshotID uuid.UUID, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	if err := rw.q.DeleteFilesByPaths(ctx, sqlc.DeleteFilesByPathsParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		Column2:    paths,
	}); err != nil {
		return fmt.Errorf("failed to delete files by paths: %w", err)
	}

	return nil
}

// === Chunk 操作 ===

// CreateChunk はチャンクを1件作成します
func (rw *IndexRepositoryRW) CreateChunk(ctx context.Context, fileID uuid.UUID, ordinal int, startLine int, endLine int, content string, contentHash string, tokenCount int, metadata *ChunkMetadata) (*models.Chunk, error) {
	// metadataがnilの場合はデフォルト値を使用（後方互換性のため）
	if metadata == nil {
		metadata = &ChunkMetadata{
			Level:    2, // デフォルトは関数/クラスレベル
			IsLatest: true,
			ChunkKey: "", // デフォルト値（空文字列）
		}
	}

	// JSONBフィールドの準備
	imports := JSONBFromStringSlice(metadata.Imports)
	calls := JSONBFromStringSlice(metadata.Calls)
	standardImports := JSONBFromStringSlice(metadata.StandardImports)
	externalImports := JSONBFromStringSlice(metadata.ExternalImports)
	internalCalls := JSONBFromStringSlice(metadata.InternalCalls)
	externalCalls := JSONBFromStringSlice(metadata.ExternalCalls)
	typeDependencies := JSONBFromStringSlice(metadata.TypeDependencies)

	chunk, err := rw.q.CreateChunk(ctx, sqlc.CreateChunkParams{
		FileID:      UUIDToPgtype(fileID),
		Ordinal:     int32(ordinal),
		StartLine:   int32(startLine),
		EndLine:     int32(endLine),
		Content:     content,
		ContentHash: contentHash,
		TokenCount:  IntToPgtype(tokenCount),
		// 構造メタデータ
		ChunkType:            StringPtrToPgtext(metadata.Type),
		ChunkName:            StringPtrToPgtext(metadata.Name),
		ParentName:           StringPtrToPgtext(metadata.ParentName),
		Signature:            StringPtrToPgtext(metadata.Signature),
		DocComment:           StringPtrToPgtext(metadata.DocComment),
		Imports:              imports,
		Calls:                calls,
		LinesOfCode:          IntPtrToPgInt4(metadata.LinesOfCode),
		CommentRatio:         Float64PtrToPgNumeric(metadata.CommentRatio),
		CyclomaticComplexity: IntPtrToPgInt4(metadata.CyclomaticComplexity),
		EmbeddingContext:     StringPtrToPgtext(metadata.EmbeddingContext),
		// 階層関係と重要度
		Level:           int32(metadata.Level),
		ImportanceScore: Float64PtrToPgNumeric(metadata.ImportanceScore),
		// 詳細な依存関係情報
		StandardImports:  standardImports,
		ExternalImports:  externalImports,
		InternalCalls:    internalCalls,
		ExternalCalls:    externalCalls,
		TypeDependencies: typeDependencies,
		// トレーサビリティ・バージョン管理
		SourceSnapshotID: UUIDPtrToPgtype(metadata.SourceSnapshotID),
		GitCommitHash:    StringPtrToPgtext(metadata.GitCommitHash),
		Author:           StringPtrToPgtext(metadata.Author),
		UpdatedAt:        TimePtrToPgtimestamp(metadata.UpdatedAt),
		IndexedAt:        TimeToPgtimestamp(time.Now()),
		FileVersion:      StringPtrToPgtext(metadata.FileVersion),
		IsLatest:         metadata.IsLatest,
		ChunkKey:         metadata.ChunkKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chunk: %w", err)
	}

	return convertSQLCChunk(chunk), nil
}

// BatchCreateChunks はチャンクを CopyFrom で一括登録します
func (rw *IndexRepositoryRW) BatchCreateChunks(ctx context.Context, chunks []*models.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	rows := make([]sqlc.CreateChunkBatchParams, 0, len(chunks))
	for _, chunk := range chunks {
		rows = append(rows, sqlc.CreateChunkBatchParams{
			FileID:      UUIDToPgtype(chunk.FileID),
			Ordinal:     int32(chunk.Ordinal),
			StartLine:   int32(chunk.StartLine),
			EndLine:     int32(chunk.EndLine),
			Content:     chunk.Content,
			ContentHash: chunk.ContentHash,
			TokenCount:  IntToPgtype(chunk.TokenCount),
		})
	}

	if _, err := rw.q.CreateChunkBatch(ctx, rows); err != nil {
		return fmt.Errorf("failed to batch create chunks: %w", err)
	}

	return nil
}

// GetChunkByID はチャンクを取得します
func (r *IndexRepositoryR) GetChunkByID(ctx context.Context, id uuid.UUID) (*models.Chunk, error) {
	chunk, err := r.q.GetChunk(ctx, UUIDToPgtype(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("chunk not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	return convertSQLCChunk(chunk), nil
}

// ListChunksByFile はファイル内のチャンクを ordinal 順に取得します
func (r *IndexRepositoryR) ListChunksByFile(ctx context.Context, fileID uuid.UUID) ([]*models.Chunk, error) {
	rows, err := r.q.ListChunksByFile(ctx, UUIDToPgtype(fileID))
	if err != nil {
		return nil, fmt.Errorf("failed to list chunks: %w", err)
	}

	chunks := make([]*models.Chunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertSQLCChunk(row))
	}

	return chunks, nil
}

// GetChunkContext は対象チャンクの前後コンテキストを取得します
func (r *IndexRepositoryR) GetChunkContext(ctx context.Context, chunkID uuid.UUID, beforeCount int, afterCount int) ([]*models.Chunk, error) {
	target, err := r.q.GetChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("chunk not found: %s", chunkID)
		}
		return nil, fmt.Errorf("failed to get target chunk: %w", err)
	}

	minOrdinal := target.Ordinal - int32(beforeCount)
	if minOrdinal < 0 {
		minOrdinal = 0
	}
	maxOrdinal := target.Ordinal + int32(afterCount)

	rows, err := r.q.ListChunksByOrdinalRange(ctx, sqlc.ListChunksByOrdinalRangeParams{
		FileID:    target.FileID,
		Ordinal:   minOrdinal,
		Ordinal_2: maxOrdinal,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get context chunks: %w", err)
	}

	chunks := make([]*models.Chunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertSQLCChunk(row))
	}

	return chunks, nil
}

// DeleteChunksByFileID はファイル配下のチャンクを削除します
func (rw *IndexRepositoryRW) DeleteChunksByFileID(ctx context.Context, fileID uuid.UUID) error {
	if err := rw.q.DeleteChunksByFile(ctx, UUIDToPgtype(fileID)); err != nil {
		return fmt.Errorf("failed to delete chunks by file: %w", err)
	}
	return nil
}

// === Embedding 操作 ===

// CreateEmbedding は単一 Embedding を作成します
func (rw *IndexRepositoryRW) CreateEmbedding(ctx context.Context, chunkID uuid.UUID, vector []float32, model string) error {
	_, err := rw.q.CreateEmbedding(ctx, sqlc.CreateEmbeddingParams{
		ChunkID: UUIDToPgtype(chunkID),
		Vector:  pgvector.NewVector(vector),
		Model:   model,
	})
	if err != nil {
		return fmt.Errorf("failed to create embedding: %w", err)
	}
	return nil
}

// BatchCreateEmbeddings は Embedding を一括登録します
func (rw *IndexRepositoryRW) BatchCreateEmbeddings(ctx context.Context, embeddings []*models.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	rows := make([]sqlc.CreateEmbeddingBatchParams, 0, len(embeddings))
	for _, embedding := range embeddings {
		rows = append(rows, sqlc.CreateEmbeddingBatchParams{
			ChunkID: UUIDToPgtype(embedding.ChunkID),
			Vector:  pgvector.NewVector(embedding.Vector),
			Model:   embedding.Model,
		})
	}

	if _, err := rw.q.CreateEmbeddingBatch(ctx, rows); err != nil {
		return fmt.Errorf("failed to batch create embeddings: %w", err)
	}

	return nil
}

// SearchByProduct はプロダクト単位でベクトル検索を実行します
func (r *IndexRepositoryR) SearchByProduct(ctx context.Context, productID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*models.SearchResult, error) {
	rows, err := r.q.SearchChunksByProduct(ctx, sqlc.SearchChunksByProductParams{
		QueryVector: pgvector.NewVector(queryVector),
		ProductID:   UUIDToPgtype(productID),
		PathPrefix:  StringPtrToPgtext(filters.PathPrefix),
		ContentType: StringPtrToPgtext(filters.ContentType),
		RowLimit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search by product: %w", err)
	}

	results := make([]*models.SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &models.SearchResult{
			ChunkID:   PgtypeToUUID(row.ChunkID),
			FilePath:  row.Path,
			StartLine: int(row.StartLine),
			EndLine:   int(row.EndLine),
			Content:   row.Content,
			Score:     row.Score,
		})
	}

	return results, nil
}

// SearchBySource はソース単位でベクトル検索を実行します
func (r *IndexRepositoryR) SearchBySource(ctx context.Context, sourceID uuid.UUID, queryVector []float32, limit int, filters SearchFilter) ([]*models.SearchResult, error) {
	rows, err := r.q.SearchChunksBySource(ctx, sqlc.SearchChunksBySourceParams{
		QueryVector: pgvector.NewVector(queryVector),
		SourceID:    UUIDToPgtype(sourceID),
		PathPrefix:  StringPtrToPgtext(filters.PathPrefix),
		ContentType: StringPtrToPgtext(filters.ContentType),
		RowLimit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search by source: %w", err)
	}

	results := make([]*models.SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &models.SearchResult{
			ChunkID:   PgtypeToUUID(row.ChunkID),
			FilePath:  row.Path,
			StartLine: int(row.StartLine),
			EndLine:   int(row.EndLine),
			Content:   row.Content,
			Score:     row.Score,
		})
	}

	return results, nil
}

// === Private helpers ===

func convertSQLCFile(row sqlc.File) *models.File {
	return &models.File{
		ID:          PgtypeToUUID(row.ID),
		SnapshotID:  PgtypeToUUID(row.SnapshotID),
		Path:        row.Path,
		Size:        row.Size,
		ContentType: row.ContentType,
		ContentHash: row.ContentHash,
		Language:    PgtextToStringPtr(row.Language),
		Domain:      PgtextToStringPtr(row.Domain),
		CreatedAt:   PgtypeToTime(row.CreatedAt),
	}
}

func convertSQLCSnapshotFile(row sqlc.SnapshotFile) *models.SnapshotFile {
	return &models.SnapshotFile{
		ID:         PgtypeToUUID(row.ID),
		SnapshotID: PgtypeToUUID(row.SnapshotID),
		FilePath:   row.FilePath,
		FileSize:   row.FileSize,
		Domain:     PgtextToStringPtr(row.Domain),
		Indexed:    row.Indexed,
		SkipReason: PgtextToStringPtr(row.SkipReason),
		CreatedAt:  PgtypeToTime(row.CreatedAt),
	}
}

func convertSQLCChunk(row sqlc.Chunk) *models.Chunk {
	return &models.Chunk{
		ID:          PgtypeToUUID(row.ID),
		FileID:      PgtypeToUUID(row.FileID),
		Ordinal:     int(row.Ordinal),
		StartLine:   int(row.StartLine),
		EndLine:     int(row.EndLine),
		Content:     row.Content,
		ContentHash: row.ContentHash,
		TokenCount:  PgtypeToInt(row.TokenCount),
		CreatedAt:   PgtypeToTime(row.CreatedAt),
		// 構造メタデータ
		Type:                 PgtextToStringPtr(row.ChunkType),
		Name:                 PgtextToStringPtr(row.ChunkName),
		ParentName:           PgtextToStringPtr(row.ParentName),
		Signature:            PgtextToStringPtr(row.Signature),
		DocComment:           PgtextToStringPtr(row.DocComment),
		Imports:              StringSliceFromJSONB(row.Imports),
		Calls:                StringSliceFromJSONB(row.Calls),
		LinesOfCode:          PgtypeToIntPtr(row.LinesOfCode),
		CommentRatio:         PgtypeToFloat64Ptr(row.CommentRatio),
		CyclomaticComplexity: PgtypeToIntPtr(row.CyclomaticComplexity),
		EmbeddingContext:     PgtextToStringPtr(row.EmbeddingContext),
		// 階層関係と重要度
		Level:           int(row.Level),
		ImportanceScore: PgtypeToFloat64Ptr(row.ImportanceScore),
		// 詳細な依存関係情報
		StandardImports:  StringSliceFromJSONB(row.StandardImports),
		ExternalImports:  StringSliceFromJSONB(row.ExternalImports),
		InternalCalls:    StringSliceFromJSONB(row.InternalCalls),
		ExternalCalls:    StringSliceFromJSONB(row.ExternalCalls),
		TypeDependencies: StringSliceFromJSONB(row.TypeDependencies),
		// トレーサビリティ・バージョン管理
		SourceSnapshotID: PgtypeToUUIDPtr(row.SourceSnapshotID),
		GitCommitHash:    PgtextToStringPtr(row.GitCommitHash),
		Author:           PgtextToStringPtr(row.Author),
		UpdatedAt:        PgtypeToTimePtr(row.UpdatedAt),
		IndexedAt:        PgtypeToTime(row.IndexedAt),
		FileVersion:      PgtextToStringPtr(row.FileVersion),
		IsLatest:         row.IsLatest,
		// 決定的な識別子
		ChunkKey: row.ChunkKey,
	}
}

// === Chunk Hierarchy 操作 ===

// AddChunkRelation は親子関係を chunk_hierarchy に追加します
func (rw *IndexRepositoryRW) AddChunkRelation(ctx context.Context, parentID, childID uuid.UUID, ordinal int) error {
	if err := rw.q.AddChunkRelation(ctx, sqlc.AddChunkRelationParams{
		ParentChunkID: UUIDToPgtype(parentID),
		ChildChunkID:  UUIDToPgtype(childID),
		Ordinal:       int32(ordinal),
	}); err != nil {
		return fmt.Errorf("failed to add chunk relation: %w", err)
	}
	return nil
}

// RemoveChunkRelation は親子関係を chunk_hierarchy から削除します
func (rw *IndexRepositoryRW) RemoveChunkRelation(ctx context.Context, parentID, childID uuid.UUID) error {
	if err := rw.q.RemoveChunkRelation(ctx, sqlc.RemoveChunkRelationParams{
		ParentChunkID: UUIDToPgtype(parentID),
		ChildChunkID:  UUIDToPgtype(childID),
	}); err != nil {
		return fmt.Errorf("failed to remove chunk relation: %w", err)
	}
	return nil
}

// GetChildChunkIDs は子チャンクのIDリストを ordinal 順で取得します
func (r *IndexRepositoryR) GetChildChunkIDs(ctx context.Context, parentID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.q.GetChildChunkIDs(ctx, UUIDToPgtype(parentID))
	if err != nil {
		return nil, fmt.Errorf("failed to get child chunk IDs: %w", err)
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, PgtypeToUUID(row))
	}

	return ids, nil
}

// GetParentChunkID は親チャンクのIDを取得します（親がいない場合は nil）
func (r *IndexRepositoryR) GetParentChunkID(ctx context.Context, chunkID uuid.UUID) (*uuid.UUID, error) {
	parentID, err := r.q.GetParentChunkID(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // 親がいない場合
		}
		return nil, fmt.Errorf("failed to get parent chunk ID: %w", err)
	}

	id := PgtypeToUUID(parentID)
	return &id, nil
}

// GetChildChunks は子チャンクのリストを ordinal 順で取得します（Chunkエンティティを結合）
func (r *IndexRepositoryR) GetChildChunks(ctx context.Context, parentID uuid.UUID) ([]*models.Chunk, error) {
	rows, err := r.q.GetChildChunks(ctx, UUIDToPgtype(parentID))
	if err != nil {
		return nil, fmt.Errorf("failed to get child chunks: %w", err)
	}

	chunks := make([]*models.Chunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, convertSQLCChunk(row))
	}

	return chunks, nil
}

// GetParentChunk は親チャンクを取得します
func (r *IndexRepositoryR) GetParentChunk(ctx context.Context, chunkID uuid.UUID) (*models.Chunk, error) {
	chunk, err := r.q.GetParentChunk(ctx, UUIDToPgtype(chunkID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // 親がいない場合
		}
		return nil, fmt.Errorf("failed to get parent chunk: %w", err)
	}

	return convertSQLCChunk(chunk), nil
}

// GetChunkTree は親→子→孫を再帰的に取得します
// 再帰CTEはsqlcで扱いにくいため、Go側で複数回クエリして実装
func (r *IndexRepositoryR) GetChunkTree(ctx context.Context, rootID uuid.UUID, maxDepth int) ([]*models.Chunk, error) {
	result := make([]*models.Chunk, 0)
	visited := make(map[uuid.UUID]bool)

	var traverse func(parentID uuid.UUID, depth int) error
	traverse = func(parentID uuid.UUID, depth int) error {
		if depth > maxDepth {
			return nil
		}
		if visited[parentID] {
			return nil // 循環参照を防止
		}
		visited[parentID] = true

		// 親チャンクを取得
		parent, err := r.GetChunkByID(ctx, parentID)
		if err != nil {
			return err
		}
		result = append(result, parent)

		// 子チャンクを取得
		children, err := r.GetChildChunks(ctx, parentID)
		if err != nil {
			return err
		}

		// 再帰的に子をたどる
		for _, child := range children {
			if err := traverse(child.ID, depth+1); err != nil {
				return err
			}
		}

		return nil
	}

	if err := traverse(rootID, 1); err != nil {
		return nil, fmt.Errorf("failed to get chunk tree: %w", err)
	}

	return result, nil
}

// === ドメイン統計 ===

// DomainCoverage はドメイン別のカバレッジ統計を表します
type DomainCoverage struct {
	Domain     string
	FileCount  int64
	ChunkCount int64
}

// GetDomainCoverageBySnapshot はスナップショット配下のドメイン別統計を取得します
func (r *IndexRepositoryR) GetDomainCoverageBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*DomainCoverage, error) {
	rows, err := r.q.GetDomainCoverageBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to get domain coverage: %w", err)
	}

	coverages := make([]*DomainCoverage, 0, len(rows))
	for _, row := range rows {
		// ChunkCountは interface{} 型なので型アサーションが必要
		var chunkCount int64
		if row.ChunkCount != nil {
			// PostgreSQLから返ってくるのは int64
			if val, ok := row.ChunkCount.(int64); ok {
				chunkCount = val
			}
		}

		coverages = append(coverages, &DomainCoverage{
			Domain:     row.Domain,
			FileCount:  row.FileCount,
			ChunkCount: chunkCount,
		})
	}

	return coverages, nil
}

// GetFilesByDomain は指定したドメインのファイル一覧を取得します
func (r *IndexRepositoryR) GetFilesByDomain(ctx context.Context, snapshotID uuid.UUID, domain string) ([]*models.File, error) {
	rows, err := r.q.GetFilesByDomain(ctx, sqlc.GetFilesByDomainParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		Domain:     StringToNullableText(domain),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get files by domain: %w", err)
	}

	files := make([]*models.File, 0, len(rows))
	for _, row := range rows {
		files = append(files, convertSQLCFile(row))
	}

	return files, nil
}

// === SnapshotFile 操作（カバレッジマップ構築） ===

// CreateSnapshotFile はスナップショットファイルレコードを作成します
func (rw *IndexRepositoryRW) CreateSnapshotFile(ctx context.Context, snapshotID uuid.UUID, filePath string, fileSize int64, domain *string, indexed bool, skipReason *string) (*models.SnapshotFile, error) {
	sf, err := rw.q.CreateSnapshotFile(ctx, sqlc.CreateSnapshotFileParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		FilePath:   filePath,
		FileSize:   fileSize,
		Domain:     StringPtrToPgtext(domain),
		Indexed:    indexed,
		SkipReason: StringPtrToPgtext(skipReason),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot file: %w", err)
	}

	return convertSQLCSnapshotFile(sf), nil
}

// UpdateSnapshotFileIndexed はスナップショットファイルのインデックス済みフラグを更新します
func (rw *IndexRepositoryRW) UpdateSnapshotFileIndexed(ctx context.Context, snapshotID uuid.UUID, filePath string, indexed bool) error {
	err := rw.q.UpdateSnapshotFileIndexed(ctx, sqlc.UpdateSnapshotFileIndexedParams{
		SnapshotID: UUIDToPgtype(snapshotID),
		FilePath:   filePath,
		Indexed:    indexed,
	})
	if err != nil {
		return fmt.Errorf("failed to update snapshot file indexed status: %w", err)
	}
	return nil
}

// GetSnapshotFilesBySnapshot はスナップショット配下の全ファイルリストを取得します
func (r *IndexRepositoryR) GetSnapshotFilesBySnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*models.SnapshotFile, error) {
	rows, err := r.q.GetSnapshotFilesBySnapshot(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot files: %w", err)
	}

	files := make([]*models.SnapshotFile, 0, len(rows))
	for _, row := range rows {
		files = append(files, convertSQLCSnapshotFile(row))
	}

	return files, nil
}

// GetDomainCoverageStats はドメイン別のカバレッジ統計を取得します
func (r *IndexRepositoryR) GetDomainCoverageStats(ctx context.Context, snapshotID uuid.UUID) ([]models.DomainCoverage, error) {
	rows, err := r.q.GetDomainCoverageStats(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to get domain coverage stats: %w", err)
	}

	coverages := make([]models.DomainCoverage, 0, len(rows))
	for _, row := range rows {
		coverages = append(coverages, models.DomainCoverage{
			Domain:           row.Domain,
			TotalFiles:       int(row.TotalFiles),
			IndexedFiles:     int(row.IndexedFiles),
			IndexedChunks:    int(row.IndexedChunks),
			CoverageRate:     PgnumericToFloat64(row.CoverageRate),
			AvgCommentRatio:  PgnumericToFloat64(row.AvgCommentRatio),
			AvgComplexity:    PgnumericToFloat64(row.AvgComplexity),
		})
	}

	return coverages, nil
}

// GetUnindexedImportantFiles は未インデックスの重要ファイルを検出します
func (r *IndexRepositoryR) GetUnindexedImportantFiles(ctx context.Context, snapshotID uuid.UUID) ([]string, error) {
	files, err := r.q.GetUnindexedImportantFiles(ctx, UUIDToPgtype(snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to get unindexed important files: %w", err)
	}
	return files, nil
}

// === 重要度スコア操作 ===

// UpdateChunkImportanceScore はチャンクの重要度スコアを更新します
func (rw *IndexRepositoryRW) UpdateChunkImportanceScore(ctx context.Context, chunkID uuid.UUID, score float64) error {
	err := rw.q.UpdateChunkImportanceScore(ctx, sqlc.UpdateChunkImportanceScoreParams{
		ID:              UUIDToPgtype(chunkID),
		ImportanceScore: Float64ToNullableNumeric(score),
	})
	if err != nil {
		return fmt.Errorf("failed to update chunk importance score: %w", err)
	}
	return nil
}

// BatchUpdateChunkImportanceScores はチャンクの重要度スコアを一括更新します
func (rw *IndexRepositoryRW) BatchUpdateChunkImportanceScores(ctx context.Context, scores map[uuid.UUID]float64) error {
	for chunkID, score := range scores {
		if err := rw.UpdateChunkImportanceScore(ctx, chunkID, score); err != nil {
			return err
		}
	}
	return nil
}
