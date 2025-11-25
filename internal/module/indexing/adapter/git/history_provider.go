package git

import (
	"context"
	"time"

	"github.com/jinford/dev-rag/internal/module/indexing/domain"
)

// HistoryProvider は domain.GitHistoryProvider の実装です
type HistoryProvider struct {
	client *GitClient
}

// NewHistoryProvider は新しいHistoryProviderを作成します
func NewHistoryProvider(client *GitClient) *HistoryProvider {
	return &HistoryProvider{
		client: client,
	}
}

// GetFileEditFrequencies は指定期間内のファイル編集頻度を取得します
func (p *HistoryProvider) GetFileEditFrequencies(ctx context.Context, repoPath, ref string, since time.Time) (map[string]*domain.FileEditHistory, error) {
	// GitClient から FileEditFrequency を取得
	gitEditFreqs, err := p.client.GetFileEditFrequencies(ctx, repoPath, ref, since)
	if err != nil {
		return nil, err
	}

	// domain.FileEditHistory 形式に変換
	editHistory := make(map[string]*domain.FileEditHistory)
	for filePath, freq := range gitEditFreqs {
		editHistory[filePath] = &domain.FileEditHistory{
			FilePath:   freq.FilePath,
			EditCount:  freq.EditCount,
			LastEdited: freq.LastEdited,
		}
	}

	return editHistory, nil
}
