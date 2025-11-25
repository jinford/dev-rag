package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jinford/dev-rag/internal/module/indexing/adapter/filter"
	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// Provider はGitソース用のSourceProvider実装です
type Provider struct {
	gitClient       *GitClient
	gitCloneBaseDir string
	defaultBranch   string
	ignoreFilter    *filter.IgnoreFilter
}

// NewProvider は新しいGit Providerを作成します
func NewProvider(gitClient *GitClient, gitCloneBaseDir, defaultBranch string) *Provider {
	return &Provider{
		gitClient:       gitClient,
		gitCloneBaseDir: gitCloneBaseDir,
		defaultBranch:   defaultBranch,
	}
}

// GetSourceType はソースタイプを返します
func (p *Provider) GetSourceType() domain.SourceType {
	return domain.SourceTypeGit
}

// ExtractSourceName はGit URLからソース名を抽出します
// 例: git@github.com:user/repo.git -> github.com/user/repo
// 例: https://github.com:8080/user/repo.git -> github.com/user/repo
func (p *Provider) ExtractSourceName(identifier string) string {
	// GitClientのURLToDirectoryNameを利用してソース名を生成
	dirName, err := p.gitClient.URLToDirectoryName(identifier)
	if err != nil {
		// パースに失敗した場合は元の文字列から.gitを除去して返す
		return strings.TrimSuffix(identifier, ".git")
	}
	return dirName
}

// FetchDocuments はGitリポジトリからドキュメント一覧を取得します
func (p *Provider) FetchDocuments(ctx context.Context, params domain.IndexParams) ([]*domain.SourceDocument, string, error) {
	// オプションから ref を取得
	ref, ok := params.Options["ref"].(string)
	if !ok || ref == "" {
		ref = p.defaultBranch
	}

	// Git URLからディレクトリ名を生成
	dirName, err := p.gitClient.URLToDirectoryName(params.Identifier)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate directory name from URL: %w", err)
	}

	// Gitリポジトリのクローン/pull
	repoPath := filepath.Join(p.gitCloneBaseDir, dirName)
	if err := p.gitClient.CloneOrPull(ctx, params.Identifier, repoPath, ref); err != nil {
		return nil, "", fmt.Errorf("failed to clone/pull repository: %w", err)
	}

	// コミット情報を取得（バージョン識別子として使用）
	commitInfo, err := p.gitClient.GetCommitInfo(ctx, repoPath, ref)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get commit info: %w", err)
	}

	// 全ファイルの最終更新コミット情報を一括取得
	fileLastCommits, err := p.gitClient.GetFileLastCommits(ctx, repoPath, ref)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file last commits: %w", err)
	}

	// ファイル一覧を取得
	files, err := p.gitClient.ListFiles(ctx, repoPath, ref)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list files: %w", err)
	}

	// 除外フィルタを作成
	ignoreFilter, err := filter.NewIgnoreFilter(repoPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create ignore filter: %w", err)
	}
	p.ignoreFilter = ignoreFilter

	// SourceDocument形式に変換
	var documents []*domain.SourceDocument
	for _, fileInfo := range files {
		// ファイル内容を読み込み
		content, err := p.gitClient.ReadFile(ctx, repoPath, ref, fileInfo.Path)
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

		documents = append(documents, &domain.SourceDocument{
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

// CreateMetadata はGitソース用のメタデータを作成します
func (p *Provider) CreateMetadata(params domain.IndexParams) domain.SourceMetadata {
	metadata := domain.SourceMetadata{
		"url": params.Identifier,
	}

	// オプションから ref を取得
	if ref, ok := params.Options["ref"].(string); ok && ref != "" {
		metadata["default_ref"] = ref
	}

	// ローカルパスを設定（重要度スコア計算用）
	dirName, err := p.gitClient.URLToDirectoryName(params.Identifier)
	if err == nil {
		repoPath := filepath.Join(p.gitCloneBaseDir, dirName)
		metadata["localPath"] = repoPath
	}

	return metadata
}

// ShouldIgnore はドキュメントを除外すべきかを判定します
func (p *Provider) ShouldIgnore(doc *domain.SourceDocument) bool {
	if p.ignoreFilter == nil {
		return false
	}
	return p.ignoreFilter.ShouldIgnore(doc.Path)
}
