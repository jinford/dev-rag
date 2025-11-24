package analyzer

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jinford/dev-rag/pkg/indexer/embedder"
	"github.com/jinford/dev-rag/pkg/sqlc"
	"github.com/jinford/dev-rag/pkg/wiki"
	"github.com/jinford/dev-rag/pkg/wiki/summarizer"
	"github.com/jinford/dev-rag/pkg/wiki/types"
)

// RepositoryAnalyzer はリポジトリ全体を解析し、ディレクトリ構造を抽出する
type RepositoryAnalyzer struct {
	pool           *pgxpool.Pool
	llm            wiki.LLMClient
	embedder       *embedder.Embedder
	securityFilter wiki.SecurityFilter
}

// NewRepositoryAnalyzer は新しいRepositoryAnalyzerを作成する
func NewRepositoryAnalyzer(
	pool *pgxpool.Pool,
	llm wiki.LLMClient,
	embedder *embedder.Embedder,
	securityFilter wiki.SecurityFilter,
) *RepositoryAnalyzer {
	return &RepositoryAnalyzer{
		pool:           pool,
		llm:            llm,
		embedder:       embedder,
		securityFilter: securityFilter,
	}
}

// AnalyzeRepository はリポジトリ全体を解析する
// 設計方針:
// - RepositoryAnalyzer は pool のみを保持
// - 読み取り専用クエリは都度 sqlc.New(pool) を使用
// - サマライザは pool を受け取り、各自でトランザクション管理
// - この設計により、トランザクションのスコープが明確になる
func (a *RepositoryAnalyzer) AnalyzeRepository(ctx context.Context, sourceID, snapshotID uuid.UUID) error {
	// 1. 既に処理済みかチェック
	if a.isAlreadyAnalyzed(ctx, snapshotID) {
		log.Printf("repository analysis already completed for snapshot %s, skipping", snapshotID)
		return nil
	}

	// 2. ディレクトリ構造の収集（既存のfilesテーブルから構築）
	structure, err := a.collectStructure(ctx, sourceID, snapshotID)
	if err != nil {
		return fmt.Errorf("structure collection failed: %w", err)
	}

	log.Printf("collected repository structure: %d directories, %d files", len(structure.Directories), len(structure.Files))

	// 3. ディレクトリサマライザーですべてのディレクトリの要約生成
	//    （File Summaryから集約）
	//    各ディレクトリで個別にトランザクションを管理
	dirSummarizer := summarizer.NewDirectorySummarizer(a.pool, a.llm, a.embedder, a.securityFilter)
	if err := dirSummarizer.GenerateSummaries(ctx, structure); err != nil {
		return fmt.Errorf("directory summaries generation failed: %w", err)
	}

	// 4. アーキテクチャサマライザーでリポジトリ全体の要約生成
	//    （Directory Summaryから集約）
	//    コミット済みのディレクトリ要約を読み込んで処理
	archSummarizer := summarizer.NewArchitectureSummarizer(a.pool, a.llm, a.embedder, a.securityFilter)
	if err := archSummarizer.GenerateSummaries(ctx, structure); err != nil {
		return fmt.Errorf("repository summary generation failed: %w", err)
	}

	log.Printf("repository analysis completed for snapshot %s", snapshotID)
	return nil
}

// isAlreadyAnalyzed は既に解析済みかチェックする
// 4種類のアーキテクチャ要約(overview, tech_stack, data_flow, components)がすべて揃っており、
// かつディレクトリ要約も存在する場合にのみtrueを返す
func (a *RepositoryAnalyzer) isAlreadyAnalyzed(ctx context.Context, snapshotID uuid.UUID) bool {
	// 読み取り専用クエリなので、都度 sqlc.New(pool) を使用
	queries := sqlc.New(a.pool)

	// uuid.UUIDをpgtype.UUIDに変換
	var pgtypeSnapshotID pgtype.UUID
	if err := pgtypeSnapshotID.Scan(snapshotID.String()); err != nil {
		log.Printf("failed to convert snapshot ID: %v", err)
		return false
	}

	// 4種類のアーキテクチャ要約がすべて揃っているかチェック
	hasAllArchSummaries, err := queries.HasAllRequiredArchitectureSummaries(ctx, pgtypeSnapshotID)
	if err != nil {
		log.Printf("failed to check architecture summaries: %v", err)
		return false
	}
	if !hasAllArchSummaries {
		log.Printf("not all required architecture summaries exist for snapshot %s", snapshotID)
		return false
	}

	// ディレクトリ要約が存在するかチェック
	dirSummaryCount, err := queries.CountDirectorySummariesBySnapshot(ctx, pgtypeSnapshotID)
	if err != nil {
		log.Printf("failed to count directory summaries: %v", err)
		return false
	}
	if dirSummaryCount == 0 {
		log.Printf("no directory summaries exist for snapshot %s", snapshotID)
		return false
	}

	return true
}

// collectStructure はリポジトリの構造を収集する
func (a *RepositoryAnalyzer) collectStructure(ctx context.Context, sourceID, snapshotID uuid.UUID) (*types.RepoStructure, error) {
	// 読み取り専用クエリなので、都度 sqlc.New(pool) を使用
	queries := sqlc.New(a.pool)

	structure := &types.RepoStructure{
		SourceID:    sourceID,
		SnapshotID:  snapshotID,
		Directories: make([]*types.DirectoryInfo, 0),
		Files:       make([]*types.FileInfo, 0),
	}

	// uuid.UUIDをpgtype.UUIDに変換
	var pgtypeSnapshotID pgtype.UUID
	if err := pgtypeSnapshotID.Scan(snapshotID.String()); err != nil {
		return nil, fmt.Errorf("failed to convert snapshot ID: %w", err)
	}

	// 既存のfilesテーブルから情報を取得
	files, err := queries.ListFilesBySnapshot(ctx, pgtypeSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// SecurityFilterでファイルパスをフィルタリング
	filteredFiles := make([]sqlc.File, 0, len(files))
	for _, file := range files {
		if !a.securityFilter.ShouldExclude(file.Path) {
			filteredFiles = append(filteredFiles, file)
		}
	}

	// ファイル情報を構築
	for _, file := range filteredFiles {
		// pgtype.UUIDをuuid.UUIDに変換
		var fileID uuid.UUID
		if err := fileID.UnmarshalBinary(file.ID.Bytes[:]); err != nil {
			log.Printf("warning: failed to convert file ID: %v", err)
			continue
		}

		fileInfo := &types.FileInfo{
			FileID:   fileID,
			Path:     file.Path,
			Size:     file.Size,
			Language: file.Language.String, // pgtype.Textからstringに変換
			Domain:   file.Domain.String,   // pgtype.Textからstringに変換
			Hash:     file.ContentHash,
		}
		structure.Files = append(structure.Files, fileInfo)
	}

	// ディレクトリ構造を構築
	structure.Directories = a.buildDirectoryStructure(structure.Files)

	return structure, nil
}

// buildDirectoryStructure はファイルリストからディレクトリ構造を構築する
func (a *RepositoryAnalyzer) buildDirectoryStructure(files []*types.FileInfo) []*types.DirectoryInfo {
	// すべてのディレクトリを抽出して構造化
	dirMap := make(map[string]*types.DirectoryInfo)

	// すべてのファイルからディレクトリを抽出
	for _, file := range files {
		dir := filepath.Dir(file.Path)

		// ルートディレクトリの正規化
		if dir == "." || dir == "" {
			dir = "."
		}

		// ディレクトリパスを分割（"." を除く）
		parts := []string{}
		if dir != "." {
			parts = strings.Split(dir, string(filepath.Separator))
		}

		// ルートディレクトリを初期化
		if _, exists := dirMap["."]; !exists {
			dirMap["."] = &types.DirectoryInfo{
				Path:           ".",
				ParentPath:     "",
				Depth:          0,
				Files:          []string{},
				Subdirectories: []string{},
				Languages:      make(map[string]int),
			}
		}

		// ルート直下のファイル
		if dir == "." {
			dirMap["."].Files = append(dirMap["."].Files, file.Path)
			dirMap["."].Languages[file.Language]++
			continue
		}

		// すべての階層のディレクトリを記録
		for depth := 1; depth <= len(parts); depth++ {
			dirPath := filepath.Join(parts[:depth]...)
			parentPath := "."
			if depth > 1 {
				parentPath = filepath.Join(parts[:depth-1]...)
			}

			if _, exists := dirMap[dirPath]; !exists {
				dirMap[dirPath] = &types.DirectoryInfo{
					Path:           dirPath,
					ParentPath:     parentPath,
					Depth:          depth,
					Files:          []string{},
					Subdirectories: []string{},
					Languages:      make(map[string]int),
				}
			}

			// このディレクトリ直下のファイル
			if len(parts) == depth {
				dirMap[dirPath].Files = append(dirMap[dirPath].Files, file.Path)
				dirMap[dirPath].Languages[file.Language]++
			}
		}
	}

	// サブディレクトリの関係を構築
	for path, dir := range dirMap {
		if dir.ParentPath != "" {
			if parent, exists := dirMap[dir.ParentPath]; exists {
				parent.Subdirectories = append(parent.Subdirectories, path)
			}
		} else if path != "." {
			// ルート直下のディレクトリ
			if root, exists := dirMap["."]; exists {
				root.Subdirectories = append(root.Subdirectories, path)
			}
		}
	}

	// 配下のすべてのファイル数を計算
	for _, dir := range dirMap {
		dir.TotalFiles = a.countTotalFiles(dir, dirMap)
	}

	// すべてのディレクトリを返す（フィルタリングなし）
	directories := make([]*types.DirectoryInfo, 0, len(dirMap))
	for _, dir := range dirMap {
		directories = append(directories, dir)
	}

	return directories
}

// countTotalFiles はディレクトリ配下のすべてのファイル数を再帰的に計算する
func (a *RepositoryAnalyzer) countTotalFiles(dir *types.DirectoryInfo, dirMap map[string]*types.DirectoryInfo) int {
	total := len(dir.Files)
	for _, subdir := range dir.Subdirectories {
		if sd, exists := dirMap[subdir]; exists {
			total += a.countTotalFiles(sd, dirMap)
		}
	}
	return total
}
