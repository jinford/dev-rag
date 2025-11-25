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

	indexingsqlc "github.com/jinford/dev-rag/internal/module/indexing/adapter/pg/sqlc"
	"github.com/jinford/dev-rag/internal/module/wiki/domain"
)

// repositoryAnalyzer は domain.RepositoryAnalyzer の実装です。
// リポジトリの構造情報を indexing モジュールの DB から収集します。
type repositoryAnalyzer struct {
	pool           *pgxpool.Pool
	securityFilter domain.SecurityFilter
}

// NewRepositoryAnalyzer は domain.RepositoryAnalyzer を実装した新しい Analyzer を作成します。
func NewRepositoryAnalyzer(
	pool *pgxpool.Pool,
	securityFilter domain.SecurityFilter,
) domain.RepositoryAnalyzer {
	return &repositoryAnalyzer{
		pool:           pool,
		securityFilter: securityFilter,
	}
}

// AnalyzeRepository はリポジトリ構造を収集して返します。
func (a *repositoryAnalyzer) AnalyzeRepository(ctx context.Context, sourceID, snapshotID uuid.UUID) (*domain.RepoStructure, error) {
	structure, err := a.collectStructure(ctx, sourceID, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("structure collection failed: %w", err)
	}

	log.Printf("collected repository structure: %d directories, %d files", len(structure.Directories), len(structure.Files))

	return structure, nil
}

// collectStructure はリポジトリの構造を収集する
func (a *repositoryAnalyzer) collectStructure(ctx context.Context, sourceID, snapshotID uuid.UUID) (*domain.RepoStructure, error) {
	// 読み取り専用クエリなので、都度 sqlc.New(pool) を使用
	indexingQueries := indexingsqlc.New(a.pool)

	structure := &domain.RepoStructure{
		SourceID:    sourceID,
		SnapshotID:  snapshotID,
		Directories: make([]*domain.DirectoryInfo, 0),
		Files:       make([]*domain.FileInfo, 0),
	}

	// uuid.UUIDをpgtype.UUIDに変換
	var pgtypeSnapshotID pgtype.UUID
	if err := pgtypeSnapshotID.Scan(snapshotID.String()); err != nil {
		return nil, fmt.Errorf("failed to convert snapshot ID: %w", err)
	}

	// 既存のfilesテーブルから情報を取得
	files, err := indexingQueries.ListFilesBySnapshot(ctx, pgtypeSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// SecurityFilterでファイルパスをフィルタリング
	filteredFiles := make([]indexingsqlc.File, 0, len(files))
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

		fileInfo := &domain.FileInfo{
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
func (a *repositoryAnalyzer) buildDirectoryStructure(files []*domain.FileInfo) []*domain.DirectoryInfo {
	// すべてのディレクトリを抽出して構造化
	dirMap := make(map[string]*domain.DirectoryInfo)

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
			dirMap["."] = &domain.DirectoryInfo{
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
				dirMap[dirPath] = &domain.DirectoryInfo{
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
	directories := make([]*domain.DirectoryInfo, 0, len(dirMap))
	for _, dir := range dirMap {
		directories = append(directories, dir)
	}

	return directories
}

// countTotalFiles はディレクトリ配下のすべてのファイル数を再帰的に計算する
func (a *repositoryAnalyzer) countTotalFiles(dir *domain.DirectoryInfo, dirMap map[string]*domain.DirectoryInfo) int {
	total := len(dir.Files)
	for _, subdir := range dir.Subdirectories {
		if sd, exists := dirMap[subdir]; exists {
			total += a.countTotalFiles(sd, dirMap)
		}
	}
	return total
}
