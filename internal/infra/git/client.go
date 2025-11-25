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

// Client は Git リポジトリ操作を提供する
type Client struct {
	sshKeyPath  string
	sshPassword string
}

// NewClient は新しい Client を作成する
func NewClient(sshKeyPath, sshPassword string) *Client {
	return &Client{
		sshKeyPath:  sshKeyPath,
		sshPassword: sshPassword,
	}
}

// CommitInfo はコミット情報を表す
type CommitInfo struct {
	Hash    string
	Date    time.Time
	Message string
	Author  string
}

// FileInfo はファイル情報を表す
type FileInfo struct {
	Path        string
	Size        int64
	ContentHash string
}

// FileEditFrequency はファイルの編集頻度情報を表す
type FileEditFrequency struct {
	FilePath   string
	EditCount  int
	LastEdited time.Time
}

// URLToDirectoryName はGit URLをディレクトリ名に変換する
func (c *Client) URLToDirectoryName(gitURL string) (string, error) {
	u, err := giturls.Parse(gitURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse git URL: %w", err)
	}

	hostname := u.Hostname()
	if hostname == "" {
		hostname = u.Host
	}

	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	return filepath.Join(hostname, path), nil
}

// Clone は Git リポジトリをクローンする
func (c *Client) Clone(ctx context.Context, url, destDir string) error {
	auth, err := c.getSSHAuth()
	if err != nil {
		return fmt.Errorf("failed to setup SSH auth: %w", err)
	}

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

// Pull は指定された ref を pull する
func (c *Client) Pull(ctx context.Context, repoPath, ref string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	auth, err := c.getSSHAuth()
	if err != nil {
		return fmt.Errorf("failed to setup SSH auth: %w", err)
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get remote: %w", err)
	}

	err = remote.FetchContext(ctx, &git.FetchOptions{
		Auth:     auth,
		Progress: os.Stdout,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewRemoteReferenceName("origin", ref),
		Force:  true,
	})
	if err != nil {
		return fmt.Errorf("failed to checkout: %w", err)
	}

	return nil
}

// CloneOrPull はリポジトリが存在しない場合はクローン、存在する場合は pull する
func (c *Client) CloneOrPull(ctx context.Context, url, destDir, ref string) error {
	gitDir := filepath.Join(destDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if err := c.Clone(ctx, url, destDir); err != nil {
			return err
		}
		return nil
	}

	return c.Pull(ctx, destDir, ref)
}

// GetCommitHash は指定された ref のコミットハッシュを取得する
func (c *Client) GetCommitHash(ctx context.Context, repoPath, ref string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return "", err
	}

	return hash.String(), nil
}

// GetCommitInfo は指定された ref のコミット情報を取得する
func (c *Client) GetCommitInfo(ctx context.Context, repoPath, ref string) (*CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return nil, err
	}

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

// GetFileLastCommits は全ファイルの最終更新コミット情報を一括取得する
func (c *Client) GetFileLastCommits(ctx context.Context, repoPath, ref string) (map[string]*CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return nil, err
	}

	commitIter, err := repo.Log(&git.LogOptions{
		From: hash,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}
	defer commitIter.Close()

	fileLastCommits := make(map[string]*CommitInfo)

	err = commitIter.ForEach(func(commit *object.Commit) error {
		tree, err := commit.Tree()
		if err != nil {
			return fmt.Errorf("failed to get tree for commit %s: %w", commit.Hash, err)
		}

		err = tree.Files().ForEach(func(f *object.File) error {
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

// ListFiles は指定された ref のファイル一覧を取得する
func (c *Client) ListFiles(ctx context.Context, repoPath, ref string) ([]*FileInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return nil, err
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	var files []*FileInfo
	err = tree.Files().ForEach(func(f *object.File) error {
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

// ReadFile は指定された ref のファイル内容を読み込む
func (c *Client) ReadFile(ctx context.Context, repoPath, ref, path string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return "", err
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		return "", fmt.Errorf("failed to get commit object: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get tree: %w", err)
	}

	file, err := tree.File(path)
	if err != nil {
		return "", fmt.Errorf("failed to get file %s: %w", path, err)
	}

	content, err := file.Contents()
	if err != nil {
		return "", fmt.Errorf("failed to read file contents: %w", err)
	}

	return content, nil
}

// GetFileEditFrequencies は指定期間内のファイル編集頻度を取得する
func (c *Client) GetFileEditFrequencies(ctx context.Context, repoPath, ref string, since time.Time) (map[string]*FileEditFrequency, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	hash, err := c.resolveRef(repo, ref)
	if err != nil {
		return nil, err
	}

	commitIter, err := repo.Log(&git.LogOptions{
		From:  hash,
		Since: &since,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}
	defer commitIter.Close()

	editFrequencies := make(map[string]*FileEditFrequency)

	err = commitIter.ForEach(func(commit *object.Commit) error {
		parents := commit.Parents()
		defer parents.Close()

		parent, err := parents.Next()
		if err != nil {
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

func (c *Client) getSSHAuth() (*ssh.PublicKeys, error) {
	if c.sshKeyPath == "" {
		return nil, nil
	}

	if _, err := os.Stat(c.sshKeyPath); os.IsNotExist(err) {
		return nil, nil
	}

	auth, err := ssh.NewPublicKeysFromFile("git", c.sshKeyPath, c.sshPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key: %w", err)
	}

	return auth, nil
}

func (c *Client) resolveRef(repo *git.Repository, ref string) (plumbing.Hash, error) {
	branchRef, err := repo.Reference(plumbing.NewBranchReferenceName(ref), true)
	if err == nil {
		return branchRef.Hash(), nil
	}

	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", ref), true)
	if err == nil {
		return remoteRef.Hash(), nil
	}

	tagRef, err := repo.Reference(plumbing.NewTagReferenceName(ref), true)
	if err == nil {
		return tagRef.Hash(), nil
	}

	if ref == "HEAD" {
		headRef, err := repo.Head()
		if err == nil {
			return headRef.Hash(), nil
		}
	}

	hash := plumbing.NewHash(ref)
	if !hash.IsZero() {
		_, err := repo.CommitObject(hash)
		if err == nil {
			return hash, nil
		}
	}

	return plumbing.ZeroHash, fmt.Errorf("failed to resolve ref: %s", ref)
}
