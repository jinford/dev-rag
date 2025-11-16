package git

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// GitClient は Git リポジトリ操作を提供します
type GitClient struct {
	// SSH認証用の秘密鍵パス
	sshKeyPath string
}

// NewGitClient は新しいGitClientを作成します
func NewGitClient(sshKeyPath string) *GitClient {
	return &GitClient{
		sshKeyPath: sshKeyPath,
	}
}

// CommitInfo はコミット情報を表します
type CommitInfo struct {
	Hash    string
	Date    time.Time
	Message string
	Author  string
}

// FileInfo はファイル情報を表します
type FileInfo struct {
	Path        string
	Size        int64
	ContentHash string
}

// Clone はGitリポジトリをクローンします
func (c *GitClient) Clone(ctx context.Context, url, destDir string) error {
	// SSH認証の設定
	auth, err := c.getSSHAuth()
	if err != nil {
		return fmt.Errorf("failed to setup SSH auth: %w", err)
	}

	// クローン実行
	_, err = git.PlainCloneContext(ctx, destDir, false, &git.CloneOptions{
		URL:      url,
		Auth:     auth,
		Progress: os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	return nil
}

// Pull は指定されたrefをpullします
func (c *GitClient) Pull(ctx context.Context, repoPath, ref string) error {
	// リポジトリを開く
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// ワークツリーを取得
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// SSH認証の設定
	auth, err := c.getSSHAuth()
	if err != nil {
		return fmt.Errorf("failed to setup SSH auth: %w", err)
	}

	// リモートを取得
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get remote: %w", err)
	}

	// Fetch実行
	err = remote.FetchContext(ctx, &git.FetchOptions{
		Auth:     auth,
		Progress: os.Stdout,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	// Checkout実行
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewRemoteReferenceName("origin", ref),
		Force:  true,
	})
	if err != nil {
		return fmt.Errorf("failed to checkout: %w", err)
	}

	return nil
}

// GetCommitHash は指定されたrefのコミットハッシュを取得します
func (c *GitClient) GetCommitHash(ctx context.Context, repoPath, ref string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	// refを解決
	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return "", err
	}

	return hash.String(), nil
}

// GetCommitInfo は指定されたrefのコミット情報を取得します
func (c *GitClient) GetCommitInfo(ctx context.Context, repoPath, ref string) (*CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// refを解決
	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return nil, err
	}

	// コミットオブジェクトを取得
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	return &CommitInfo{
		Hash:    commit.Hash.String(),
		Date:    commit.Author.When,
		Message: commit.Message,
		Author:  commit.Author.Name,
	}, nil
}

// ListFiles は指定されたrefのファイル一覧を取得します
func (c *GitClient) ListFiles(ctx context.Context, repoPath, ref string) ([]*FileInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// refを解決
	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return nil, err
	}

	// コミットオブジェクトを取得
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	// ツリーを取得
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	// ファイル一覧を取得
	var files []*FileInfo
	err = tree.Files().ForEach(func(f *object.File) error {
		// ファイル内容のハッシュを計算
		reader, err := f.Reader()
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", f.Name, err)
		}
		defer reader.Close()

		hash := sha256.New()
		size, err := io.Copy(hash, reader)
		if err != nil {
			return fmt.Errorf("failed to hash file %s: %w", f.Name, err)
		}

		files = append(files, &FileInfo{
			Path:        f.Name,
			Size:        size,
			ContentHash: fmt.Sprintf("%x", hash.Sum(nil)),
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate files: %w", err)
	}

	return files, nil
}

// ReadFile は指定されたrefのファイル内容を読み込みます
func (c *GitClient) ReadFile(ctx context.Context, repoPath, ref, path string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	// refを解決
	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return "", err
	}

	// コミットオブジェクトを取得
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return "", fmt.Errorf("failed to get commit object: %w", err)
	}

	// ツリーを取得
	tree, err := commit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get tree: %w", err)
	}

	// ファイルを取得
	file, err := tree.File(path)
	if err != nil {
		return "", fmt.Errorf("failed to get file %s: %w", path, err)
	}

	// ファイル内容を読み込み
	content, err := file.Contents()
	if err != nil {
		return "", fmt.Errorf("failed to read file contents: %w", err)
	}

	return content, nil
}

// getSSHAuth はSSH認証を設定します
func (c *GitClient) getSSHAuth() (*ssh.PublicKeys, error) {
	if c.sshKeyPath == "" {
		return nil, nil
	}

	// SSH鍵が存在しない場合は認証なし
	if _, err := os.Stat(c.sshKeyPath); os.IsNotExist(err) {
		return nil, nil
	}

	// SSH鍵を読み込み
	auth, err := ssh.NewPublicKeysFromFile("git", c.sshKeyPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key: %w", err)
	}

	return auth, nil
}

// resolveRef はrefを解決してHashを返します
func (c *GitClient) resolveRef(repo *git.Repository, ref string) (plumbing.Hash, error) {
	// ブランチとして解決を試みる
	branchRef, err := repo.Reference(plumbing.NewBranchReferenceName(ref), true)
	if err == nil {
		return branchRef.Hash(), nil
	}

	// リモートブランチとして解決を試みる
	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", ref), true)
	if err == nil {
		return remoteRef.Hash(), nil
	}

	// タグとして解決を試みる
	tagRef, err := repo.Reference(plumbing.NewTagReferenceName(ref), true)
	if err == nil {
		return tagRef.Hash(), nil
	}

	// HEADとして解決を試みる
	if ref == "HEAD" {
		headRef, err := repo.Head()
		if err == nil {
			return headRef.Hash(), nil
		}
	}

	// 直接ハッシュとして解決を試みる
	hash := plumbing.NewHash(ref)
	if !hash.IsZero() {
		_, err := repo.CommitObject(hash)
		if err == nil {
			return hash, nil
		}
	}

	return plumbing.ZeroHash, fmt.Errorf("failed to resolve ref: %s", ref)
}

// CloneOrPull はリポジトリが存在しない場合はクローン、存在する場合はpullします
func (c *GitClient) CloneOrPull(ctx context.Context, url, destDir, ref string) error {
	// リポジトリディレクトリが存在するか確認
	gitDir := filepath.Join(destDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// 存在しない場合はクローン
		if err := c.Clone(ctx, url, destDir); err != nil {
			return err
		}
	}

	// pullを実行
	return c.Pull(ctx, destDir, ref)
}
