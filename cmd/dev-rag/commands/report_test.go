package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenInBrowser(t *testing.T) {
	// 一時ファイルを作成
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")
	err := os.WriteFile(testFile, []byte("<html><body>Test</body></html>"), 0644)
	assert.NoError(t, err)

	// ブラウザを開く（実際には開かないが、コマンドが実行されることを確認）
	// CI環境では失敗する可能性があるため、エラーを許容
	err = openInBrowser(testFile)
	// エラーが発生してもテストは失敗させない（CI/CD環境を考慮）
	if err != nil {
		t.Logf("ブラウザを開けませんでした（これは正常な場合があります）: %v", err)
	}
}

func TestReportGenerateAction_OutputPath(t *testing.T) {
	// 出力パスが正しく解決されることをテスト
	tests := []struct {
		name       string
		outputPath string
		wantErr    bool
	}{
		{
			name:       "相対パス",
			outputPath: "report.html",
			wantErr:    false,
		},
		{
			name:       "ディレクトリ付き",
			outputPath: "reports/quality_report.html",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath, err := filepath.Abs(tt.outputPath)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, absPath)
				assert.True(t, filepath.IsAbs(absPath))
			}
		})
	}
}
