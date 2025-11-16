package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jinford/dev-rag/pkg/indexer/filter"
	"github.com/jinford/dev-rag/pkg/indexer/provider"
	"github.com/jinford/dev-rag/pkg/models"
)

// Provider はGitソース用のSourceProvider実装です
type Provider struct {
	gitClient       *GitClient
	gitCloneBaseDir string
	defaultBranch   string
	sourceID        string // クローン先のディレクトリパス生成用
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
func (p *Provider) GetSourceType() models.SourceType {
	return models.SourceTypeGit
}

// ExtractSourceName はGit URLからソース名を抽出します
// 例: git@github.com:user/repo.git -> github.com/user/repo
// 例: https://github.com/user/repo.git -> github.com/user/repo
func (p *Provider) ExtractSourceName(identifier string) string {
	// 末尾の.gitを除去
	url := strings.TrimSuffix(identifier, ".git")

	// SSH形式（git@host:path）の場合
	if strings.Contains(url, "@") && strings.Contains(url, ":") {
		// git@github.com:user/repo の形式
		atIdx := strings.Index(url, "@")
		colonIdx := strings.Index(url[atIdx:], ":")
		if colonIdx > 0 {
			host := url[atIdx+1 : atIdx+colonIdx]
			path := url[atIdx+colonIdx+1:]
			return host + "/" + path
		}
	}

	// HTTPS形式（https://host/path）の場合
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		// プロトコル部分を除去
		url = strings.TrimPrefix(url, "https://")
		url = strings.TrimPrefix(url, "http://")
		return url
	}

	// その他の場合はそのまま返す
	return url
}

// FetchDocuments はGitリポジトリからドキュメント一覧を取得します
func (p *Provider) FetchDocuments(ctx context.Context, params provider.IndexParams) ([]*provider.SourceDocument, string, error) {
	// オプションから ref を取得
	ref, ok := params.Options["ref"].(string)
	if !ok || ref == "" {
		ref = p.defaultBranch
	}

	// sourceIDを取得（クローン先ディレクトリパス生成用）
	p.sourceID, ok = params.Options["sourceID"].(string)
	if !ok || p.sourceID == "" {
		return nil, "", fmt.Errorf("sourceID is required in options")
	}

	// Gitリポジトリのクローン/pull
	repoPath := filepath.Join(p.gitCloneBaseDir, p.sourceID)
	if err := p.gitClient.CloneOrPull(ctx, params.Identifier, repoPath, ref); err != nil {
		return nil, "", fmt.Errorf("failed to clone/pull repository: %w", err)
	}

	// コミット情報を取得
	commitInfo, err := p.gitClient.GetCommitInfo(ctx, repoPath, ref)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get commit info: %w", err)
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
	var documents []*provider.SourceDocument
	for _, fileInfo := range files {
		// ファイル内容を読み込み
		content, err := p.gitClient.ReadFile(ctx, repoPath, ref, fileInfo.Path)
		if err != nil {
			// ファイル読み込みエラーはスキップ
			continue
		}

		documents = append(documents, &provider.SourceDocument{
			Path:        fileInfo.Path,
			Content:     content,
			Size:        fileInfo.Size,
			ContentHash: fileInfo.ContentHash,
		})
	}

	return documents, commitInfo.Hash, nil
}

// CreateMetadata はGitソース用のメタデータを作成します
func (p *Provider) CreateMetadata(params provider.IndexParams) models.SourceMetadata {
	metadata := models.SourceMetadata{
		"url": params.Identifier,
	}

	// オプションから ref を取得
	if ref, ok := params.Options["ref"].(string); ok && ref != "" {
		metadata["default_ref"] = ref
	}

	return metadata
}

// ShouldIgnore はドキュメントを除外すべきかを判定します
func (p *Provider) ShouldIgnore(doc *provider.SourceDocument) bool {
	if p.ignoreFilter == nil {
		return false
	}
	return p.ignoreFilter.ShouldIgnore(doc.Path)
}
