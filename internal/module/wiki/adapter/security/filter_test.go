package security

import (
	"strings"
	"testing"
)

func TestFilter_ContainsSensitiveInfo(t *testing.T) {
	filter := NewFilter()

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "API key",
			content:  `api_key = "sk-1234567890abcdefghijklmnopqrstuvwxyz"`,
			expected: true,
		},
		{
			name:     "AWS access key",
			content:  `AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE`,
			expected: true,
		},
		{
			name:     "Private key",
			content:  `-----BEGIN RSA PRIVATE KEY-----`,
			expected: true,
		},
		{
			name:     "GitHub token",
			content:  `GITHUB_TOKEN=ghp_1234567890abcdefghijklmnopqrst`,
			expected: true,
		},
		{
			name:     "OpenAI API key",
			content:  `OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz1234567890`,
			expected: true,
		},
		{
			name:     "JWT token",
			content:  `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c`,
			expected: true,
		},
		{
			name:     "Safe content",
			content:  `package main\n\nfunc main() {\n  fmt.Println("Hello, World!")\n}`,
			expected: false,
		},
		{
			name:     "Comment about API key (not actual key)",
			content:  `// TODO: Set API key in environment variable`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.ContainsSensitiveInfo(tt.content)
			if result != tt.expected {
				t.Errorf("ContainsSensitiveInfo() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFilter_MaskSensitiveInfo(t *testing.T) {
	filter := NewFilter()

	tests := []struct {
		name     string
		content  string
		contains string // マスクされた結果に含まれるべき文字列
		notContains string // マスクされた結果に含まれないべき文字列
	}{
		{
			name:        "API key masking",
			content:     `api_key = "sk-1234567890abcdefghijklmnopqrstuvwxyz"`,
			contains:    "***MASKED***",
			notContains: "sk-1234567890",
		},
		{
			name:        "AWS key masking",
			content:     `AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE`,
			contains:    "***MASKED***",
			notContains: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:        "Private key masking",
			content:     `-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...`,
			contains:    "***MASKED***",
			notContains: "BEGIN RSA PRIVATE KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked := filter.MaskSensitiveInfo(tt.content)

			if !strings.Contains(masked, tt.contains) {
				t.Errorf("MaskSensitiveInfo() should contain %q, got %q", tt.contains, masked)
			}

			if tt.notContains != "" && strings.Contains(masked, tt.notContains) {
				t.Errorf("MaskSensitiveInfo() should not contain %q, got %q", tt.notContains, masked)
			}
		})
	}
}

func TestFilter_FilterFiles(t *testing.T) {
	filter := NewFilter()

	tests := []struct {
		name          string
		files         []string
		expectedCount int
		shouldExclude []string // これらのファイルは除外されるべき
		shouldInclude []string // これらのファイルは含まれるべき
	}{
		{
			name: "Environment files",
			files: []string{
				"main.go",
				".env",
				".env.local",
				"config.yaml",
				"test.env",
			},
			expectedCount: 2,
			shouldExclude: []string{".env", ".env.local", "test.env"},
			shouldInclude: []string{"main.go", "config.yaml"},
		},
		{
			name: "Credential files",
			files: []string{
				"README.md",
				"credentials.json",
				"secrets.yaml",
				"api_token.txt",
				"config.go",
			},
			expectedCount: 2,
			shouldExclude: []string{"credentials.json", "secrets.yaml", "api_token.txt"},
			shouldInclude: []string{"README.md", "config.go"},
		},
		{
			name: "Key files",
			files: []string{
				"server.go",
				"private.key",
				"cert.pem",
				"id_rsa",
				"id_rsa.pub",
			},
			expectedCount: 1,
			shouldExclude: []string{"private.key", "cert.pem", "id_rsa", "id_rsa.pub"},
			shouldInclude: []string{"server.go"},
		},
		{
			name: "Safe files only",
			files: []string{
				"main.go",
				"README.md",
				"config.yaml",
				"test.go",
			},
			expectedCount: 4,
			shouldExclude: []string{},
			shouldInclude: []string{"main.go", "README.md", "config.yaml", "test.go"},
		},
		{
			name: "Path with sensitive keywords",
			files: []string{
				"src/main.go",
				".ssh/config",
				"secrets/api_keys.yaml",
				"private/data.json",
				"public/index.html",
			},
			expectedCount: 2,
			shouldExclude: []string{".ssh/config", "secrets/api_keys.yaml", "private/data.json"},
			shouldInclude: []string{"src/main.go", "public/index.html"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filter.FilterFiles(tt.files)

			if len(filtered) != tt.expectedCount {
				t.Errorf("FilterFiles() returned %d files, want %d. Got: %v", len(filtered), tt.expectedCount, filtered)
			}

			// 除外されるべきファイルがフィルタリング後に含まれていないか確認
			for _, excludeFile := range tt.shouldExclude {
				for _, filteredFile := range filtered {
					if filteredFile == excludeFile {
						t.Errorf("File %q should be excluded but was included", excludeFile)
					}
				}
			}

			// 含まれるべきファイルがフィルタリング後に含まれているか確認
			for _, includeFile := range tt.shouldInclude {
				found := false
				for _, filteredFile := range filtered {
					if filteredFile == includeFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("File %q should be included but was excluded. Filtered: %v", includeFile, filtered)
				}
			}
		})
	}
}

func TestFilter_isSensitiveFile(t *testing.T) {
	filter := NewFilter()

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{name: ".env file", filePath: ".env", expected: true},
		{name: ".env.local", filePath: ".env.local", expected: true},
		{name: "production.env", filePath: "production.env", expected: true},
		{name: "credentials.json", filePath: "config/credentials.json", expected: true},
		{name: "secrets.yaml", filePath: "secrets.yaml", expected: true},
		{name: "private.key", filePath: "certs/private.key", expected: true},
		{name: "id_rsa", filePath: ".ssh/id_rsa", expected: true},
		{name: "main.go", filePath: "main.go", expected: false},
		{name: "README.md", filePath: "README.md", expected: false},
		{name: "config.yaml", filePath: "config.yaml", expected: false},
		{name: "secrets in path", filePath: "secrets/config.json", expected: true},
		{name: ".ssh in path", filePath: ".ssh/known_hosts", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.isSensitiveFile(tt.filePath)
			if result != tt.expected {
				t.Errorf("isSensitiveFile(%q) = %v, want %v", tt.filePath, result, tt.expected)
			}
		})
	}
}
