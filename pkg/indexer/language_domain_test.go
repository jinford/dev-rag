package indexer

import (
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	idx := &Indexer{}

	tests := []struct {
		name     string
		path     string
		content  string
		expected *string
	}{
		{
			name:     "Go言語ファイル",
			path:     "main.go",
			content:  "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}",
			expected: stringPtr("Go"),
		},
		{
			name:     "Pythonファイル",
			path:     "script.py",
			content:  "def main():\n    print('Hello')",
			expected: stringPtr("Python"),
		},
		{
			name:     "JavaScriptファイル",
			path:     "app.js",
			content:  "function main() {\n    console.log('Hello');\n}",
			expected: stringPtr("JavaScript"),
		},
		{
			name:     "TypeScriptファイル",
			path:     "app.ts",
			content:  "function main(): void {\n    console.log('Hello');\n}",
			expected: stringPtr("TypeScript"),
		},
		{
			name:     "Markdownファイル",
			path:     "README.md",
			content:  "# Title\n\nContent",
			expected: stringPtr("Markdown"),
		},
		{
			name:     "YAMLファイル",
			path:     "config.yml",
			content:  "version: 1.0\nname: test",
			expected: stringPtr("YAML"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := idx.detectLanguage(tt.path, tt.content)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", *result)
				}
			} else {
				if result == nil {
					t.Errorf("Expected %v, got nil", *tt.expected)
				} else if *result != *tt.expected {
					t.Errorf("Expected %v, got %v", *tt.expected, *result)
				}
			}
		})
	}
}

func TestClassifyDomain(t *testing.T) {
	idx := &Indexer{}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		// テストファイル
		{
			name:     "Goテストファイル",
			path:     "pkg/main_test.go",
			expected: "tests",
		},
		{
			name:     "テストディレクトリ内のファイル",
			path:     "test/unit/helper.go",
			expected: "tests",
		},
		{
			name:     "testsディレクトリ",
			path:     "tests/integration/api_test.go",
			expected: "tests",
		},
		{
			name:     "JavaScriptテスト",
			path:     "src/__tests__/app.test.js",
			expected: "tests",
		},
		{
			name:     "Rubyテスト",
			path:     "spec/models/user_spec.rb",
			expected: "tests",
		},
		// ドキュメント
		{
			name:     "README",
			path:     "README.md",
			expected: "architecture",
		},
		{
			name:     "docsディレクトリ",
			path:     "docs/architecture/design.md",
			expected: "architecture",
		},
		{
			name:     "docディレクトリ",
			path:     "doc/api.md",
			expected: "architecture",
		},
		{
			name:     "reStructuredText",
			path:     "docs/index.rst",
			expected: "architecture",
		},
		{
			name:     "AsciiDoc",
			path:     "docs/manual.adoc",
			expected: "architecture",
		},
		// インフラストラクチャ
		{
			name:     "Dockerfile",
			path:     "Dockerfile",
			expected: "infra",
		},
		{
			name:     "docker-compose",
			path:     "docker-compose.yml",
			expected: "infra",
		},
		{
			name:     "Kubernetes manifest",
			path:     "k8s/deployment.yaml",
			expected: "infra",
		},
		{
			name:     "Terraform",
			path:     "terraform/main.tf",
			expected: "infra",
		},
		{
			name:     "Ansible",
			path:     "ansible/playbook.yml",
			expected: "infra",
		},
		{
			name:     "YAML設定ファイル",
			path:     "config/settings.yml",
			expected: "infra",
		},
		// 運用スクリプト
		{
			name:     "シェルスクリプト",
			path:     "scripts/deploy.sh",
			expected: "ops",
		},
		{
			name:     "Bashスクリプト",
			path:     "scripts/backup.bash",
			expected: "ops",
		},
		{
			name:     "opsディレクトリ",
			path:     "ops/monitoring.py",
			expected: "ops",
		},
		// コード（デフォルト）
		{
			name:     "Goソースコード",
			path:     "pkg/indexer/indexer.go",
			expected: "code",
		},
		{
			name:     "Pythonソースコード",
			path:     "src/main.py",
			expected: "code",
		},
		{
			name:     "JavaScriptソースコード",
			path:     "src/components/App.jsx",
			expected: "code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := idx.classifyDomain(tt.path)

			if result == nil {
				t.Errorf("Expected %v, got nil", tt.expected)
			} else if *result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, *result)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
