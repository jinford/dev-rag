package git

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	giturls "github.com/whilp/git-urls"
)

// GitClient は Git リポジトリ操作を提供します
type GitClient struct {
	// SSH認証用の秘密鍵パス
	sshKeyPath string
	// SSH秘密鍵のパスワード（パスフレーズ）
	sshPassword string
}

// NewGitClient は新しいGitClientを作成します
func NewGitClient(sshKeyPath, sshPassword string) *GitClient {
	return &GitClient{
		sshKeyPath:  sshKeyPath,
		sshPassword: sshPassword,
	}
}

// URLToDirectoryName はGit URLをディレクトリ名に変換します
// 例: https://github.com/hoge/fuga.git -> github.com/hoge/fuga
// 例: git@github.com:hoge/fuga.git -> github.com/hoge/fuga
// 例: https://github.com:8080/hoge/fuga.git -> github.com/hoge/fuga
func (c *GitClient) URLToDirectoryName(gitURL string) (string, error) {
	u, err := giturls.Parse(gitURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse git URL: %w", err)
	}

	// ホスト名のみを取得（ポート番号を除外）
	hostname := u.Hostname()
	if hostname == "" {
		// Hostname()が空の場合はHostをそのまま使う
		hostname = u.Host
	}

	// パスから .git サフィックスを削除
	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	// ホスト名/パスの形式で返す
	return filepath.Join(hostname, path), nil
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

// GetFileLastCommits は全ファイルの最終更新コミット情報を一括取得します
// git log の履歴を遡り、各ファイルが最初に出現したコミットを最終更新コミットとして記録します
// パフォーマンス: O(N) の時間複雑度で動作します（ファイル数 N に対して）
func (c *GitClient) GetFileLastCommits(ctx context.Context, repoPath, ref string) (map[string]*CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// refを解決
	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return nil, err
	}

	// コミット履歴を取得（指定されたrefから）
	commitIter, err := repo.Log(&git.LogOptions{
		From: hash,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}
	defer commitIter.Close()

	// ファイルパス→最終更新コミット情報のマップ
	fileLastCommits := make(map[string]*CommitInfo)

	// コミット履歴を遡り、各ファイルの最終更新コミットを記録
	err = commitIter.ForEach(func(commit *object.Commit) error {
		// コミットのツリーを取得
		tree, err := commit.Tree()
		if err != nil {
			return fmt.Errorf("failed to get tree for commit %s: %w", commit.Hash, err)
		}

		// ツリー内の全ファイルを走査
		err = tree.Files().ForEach(func(f *object.File) error {
			// まだ記録されていないファイルの場合、このコミットを最終更新コミットとして記録
			if _, exists := fileLastCommits[f.Name]; !exists {
				fileLastCommits[f.Name] = &CommitInfo{
					Hash:    commit.Hash.String(),
					Date:    commit.Author.When,
					Message: commit.Message,
					Author:  commit.Author.Name,
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to iterate files in commit %s: %w", commit.Hash, err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return fileLastCommits, nil
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
	auth, err := ssh.NewPublicKeysFromFile("git", c.sshKeyPath, c.sshPassword)
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
		// 存在しない場合はクローン（新規クローン時はPullをスキップ）
		if err := c.Clone(ctx, url, destDir); err != nil {
			return err
		}
		return nil
	}

	// 既存リポジトリの場合はpullを実行
	return c.Pull(ctx, destDir, ref)
}

// FileEditFrequency はファイルの編集頻度情報を表します
type FileEditFrequency struct {
	FilePath   string
	EditCount  int       // 指定期間内の編集回数
	LastEdited time.Time // 最終編集日時
}

// GetFileEditFrequencies は指定期間内のファイル編集頻度を取得します
// since: 集計開始日時（例: 過去90日 = time.Now().AddDate(0, 0, -90)）
// ref: 対象ブランチ/タグ/コミット
func (c *GitClient) GetFileEditFrequencies(ctx context.Context, repoPath, ref string, since time.Time) (map[string]*FileEditFrequency, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// refを解決
	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return nil, err
	}

	// コミット履歴を取得（指定されたrefから）
	commitIter, err := repo.Log(&git.LogOptions{
		From:  hash,
		Since: &since,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}
	defer commitIter.Close()

	// ファイルパス→編集頻度情報のマップ
	editFrequencies := make(map[string]*FileEditFrequency)

	// コミット履歴を遡り、各ファイルの編集回数をカウント
	err = commitIter.ForEach(func(commit *object.Commit) error {
		// 親コミットを取得（変更差分を計算するため）
		parents := commit.Parents()
		defer parents.Close()

		parent, err := parents.Next()
		if err != nil {
			// 親がいない場合（初回コミット）はこのコミットのすべてのファイルをカウント
			tree, err := commit.Tree()
			if err != nil {
				return fmt.Errorf("failed to get tree for commit %s: %w", commit.Hash, err)
			}

			return tree.Files().ForEach(func(f *object.File) error {
				if freq, exists := editFrequencies[f.Name]; exists {
					freq.EditCount++
					if commit.Author.When.After(freq.LastEdited) {
						freq.LastEdited = commit.Author.When
					}
				} else {
					editFrequencies[f.Name] = &FileEditFrequency{
						FilePath:   f.Name,
						EditCount:  1,
						LastEdited: commit.Author.When,
					}
				}
				return nil
			})
		}

		// 親コミットとの差分を取得
		parentTree, err := parent.Tree()
		if err != nil {
			return fmt.Errorf("failed to get parent tree: %w", err)
		}

		currentTree, err := commit.Tree()
		if err != nil {
			return fmt.Errorf("failed to get current tree: %w", err)
		}

		changes, err := parentTree.Diff(currentTree)
		if err != nil {
			return fmt.Errorf("failed to diff trees: %w", err)
		}

		// 変更されたファイルをカウント
		for _, change := range changes {
			var filePath string
			if change.To.Name != "" {
				filePath = change.To.Name
			} else if change.From.Name != "" {
				filePath = change.From.Name
			} else {
				continue
			}

			if freq, exists := editFrequencies[filePath]; exists {
				freq.EditCount++
				if commit.Author.When.After(freq.LastEdited) {
					freq.LastEdited = commit.Author.When
				}
			} else {
				editFrequencies[filePath] = &FileEditFrequency{
					FilePath:   filePath,
					EditCount:  1,
					LastEdited: commit.Author.When,
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return editFrequencies, nil
}
