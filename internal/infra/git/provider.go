package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jinford/dev-rag/internal/core/ingestion"
	"github.com/jinford/dev-rag/internal/infra/git/filter"
)

// Provider は Git ソース用の ingestion.SourceProvider 実装
type Provider struct {
	client          *Client
	gitCloneBaseDir string
	defaultBranch   string
	ignoreFilter    *filter.IgnoreFilter
}

// NewProvider は新しい Git Provider を作成する
func NewProvider(client *Client, gitCloneBaseDir, defaultBranch string) *Provider {
	return &Provider{
		client:          client,
		gitCloneBaseDir: gitCloneBaseDir,
		defaultBranch:   defaultBranch,
	}
}

// GetSourceType は ingestion.SourceTypeGit を返す
func (p *Provider) GetSourceType() ingestion.SourceType {
	return ingestion.SourceTypeGit
}

// ExtractSourceName は Git URL からソース名を抽出する
// 例: git@github.com:user/repo.git -> github.com/user/repo
// 例: https://github.com:8080/user/repo.git -> github.com/user/repo
func (p *Provider) ExtractSourceName(identifier string) string {
	// Client の URLToDirectoryName を利用してソース名を生成
	dirName, err := p.client.URLToDirectoryName(identifier)
	if err != nil {
		// パースに失敗した場合は元の文字列から .git を除去して返す
		return strings.TrimSuffix(identifier, ".git")
	}
	return dirName
}

// FetchDocuments は Git リポジトリからドキュメント一覧を取得する
func (p *Provider) FetchDocuments(ctx context.Context, params ingestion.IndexParams) ([]*ingestion.SourceDocument, string, error) {
	// オプションから ref を取得
	ref, ok := params.Options["ref"].(string)
	if !ok || ref == "" {
		ref = p.defaultBranch
	}

	// Git URL からディレクトリ名を生成
	dirName, err := p.client.URLToDirectoryName(params.Identifier)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate directory name from URL: %w", err)
	}

	// Git リポジトリのクローン/pull
	repoPath := filepath.Join(p.gitCloneBaseDir, dirName)
	if err := p.client.CloneOrPull(ctx, params.Identifier, repoPath, ref); err != nil {
		return nil, "", fmt.Errorf("failed to clone/pull repository: %w", err)
	}

	// コミット情報を取得（バージョン識別子として使用）
	commitInfo, err := p.client.GetCommitInfo(ctx, repoPath, ref)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get commit info: %w", err)
	}

	// 全ファイルの最終更新コミット情報を一括取得
	fileLastCommits, err := p.client.GetFileLastCommits(ctx, repoPath, ref)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file last commits: %w", err)
	}

	// ファイル一覧を取得
	files, err := p.client.ListFiles(ctx, repoPath, ref)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list files: %w", err)
	}

	// 除外フィルタを作成
	ignoreFilter, err := filter.NewIgnoreFilter(repoPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create ignore filter: %w", err)
	}
	p.ignoreFilter = ignoreFilter

	// ingestion.SourceDocument 形式に変換
	var documents []*ingestion.SourceDocument
	for _, fileInfo := range files {
		// ファイル内容を読み込み
		content, err := p.client.ReadFile(ctx, repoPath, ref, fileInfo.Path)
		if err != nil {
			// ファイル読み込みエラーはスキップ
			continue
		}

		// マップから各ファイルのコミット情報を取得
		fileCommit, ok := fileLastCommits[fileInfo.Path]
		if !ok {
			// コミット情報が取得できなかった場合はリポジトリ全体の最新コミット情報を使用（フォールバック）
			fileCommit = commitInfo
		}

		documents = append(documents, &ingestion.SourceDocument{
			Path:        fileInfo.Path,
			Content:     content,
			Size:        fileInfo.Size,
			ContentHash: fileInfo.ContentHash,
			// ファイル固有のコミット情報を設定
			CommitHash: fileCommit.Hash,
			Author:     fileCommit.Author,
			UpdatedAt:  fileCommit.Date,
		})
	}

	return documents, commitInfo.Hash, nil
}

// CreateMetadata は Git ソース用のメタデータを作成する
func (p *Provider) CreateMetadata(params ingestion.IndexParams) ingestion.SourceMetadata {
	metadata := ingestion.SourceMetadata{
		"url": params.Identifier,
	}

	// オプションから ref を取得
	if ref, ok := params.Options["ref"].(string); ok && ref != "" {
		metadata["default_ref"] = ref
	}

	// ローカルパスを設定（重要度スコア計算用）
	dirName, err := p.client.URLToDirectoryName(params.Identifier)
	if err == nil {
		repoPath := filepath.Join(p.gitCloneBaseDir, dirName)
		metadata["localPath"] = repoPath
	}

	return metadata
}

// ShouldIgnore はドキュメントを除外すべきかを判定する
func (p *Provider) ShouldIgnore(doc *ingestion.SourceDocument) bool {
	if p.ignoreFilter == nil {
		return false
	}
	return p.ignoreFilter.ShouldIgnore(doc.Path)
}
