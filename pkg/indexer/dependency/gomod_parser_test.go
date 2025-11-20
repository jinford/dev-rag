package dependency

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoModParser_ParseContent(t *testing.T) {
	content := `module github.com/jinford/dev-rag

go 1.24.4

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.6
	github.com/openai/openai-go/v3 v3.8.1
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
)
`

	parser := NewGoModParser()
	versions, err := parser.ParseContent(content)
	require.NoError(t, err)
	require.NotNil(t, versions)

	// バージョン情報の確認
	assert.Equal(t, "v1.6.0", versions["github.com/google/uuid"])
	assert.Equal(t, "v5.7.6", versions["github.com/jackc/pgx/v5"])
	assert.Equal(t, "v3.8.1", versions["github.com/openai/openai-go/v3"])
	assert.Equal(t, "v0.0.0-20230124172434-306776ec8161", versions["github.com/Azure/go-ansiterm"])
	assert.Equal(t, "v0.6.2", versions["github.com/Microsoft/go-winio"])
}

func TestGoModParser_ParseSingleRequire(t *testing.T) {
	content := `module github.com/jinford/dev-rag

go 1.24.4

require github.com/google/uuid v1.6.0
`

	parser := NewGoModParser()
	versions, err := parser.ParseContent(content)
	require.NoError(t, err)

	assert.Equal(t, "v1.6.0", versions["github.com/google/uuid"])
}

func TestGoModParser_ParseWithComments(t *testing.T) {
	content := `module github.com/jinford/dev-rag

go 1.24.4

require (
	// Testing libraries
	github.com/stretchr/testify v1.11.1
	github.com/google/uuid v1.6.0 // UUID generation
)
`

	parser := NewGoModParser()
	versions, err := parser.ParseContent(content)
	require.NoError(t, err)

	assert.Equal(t, "v1.11.1", versions["github.com/stretchr/testify"])
	assert.Equal(t, "v1.6.0", versions["github.com/google/uuid"])
}

func TestGoModParser_GetModulePath(t *testing.T) {
	content := `module github.com/jinford/dev-rag

go 1.24.4
`

	parser := NewGoModParser()
	modulePath := parser.GetModulePath(content)

	assert.Equal(t, "github.com/jinford/dev-rag", modulePath)
}

func TestGoModParser_EmptyContent(t *testing.T) {
	content := ""

	parser := NewGoModParser()
	versions, err := parser.ParseContent(content)
	require.NoError(t, err)

	assert.Equal(t, 0, len(versions))
}

func TestIsStandardLibrary(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		expected bool
	}{
		{"fmt", "fmt", true},
		{"strings", "strings", true},
		{"net/http", "net/http", true},
		{"golang.org/x/crypto", "golang.org/x/crypto", true},
		{"github.com/google/uuid", "github.com/google/uuid", false},
		{"example.com/pkg", "example.com/pkg", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsStandardLibrary(tt.pkg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetVersion(t *testing.T) {
	versions := map[string]string{
		"github.com/google/uuid":  "v1.6.0",
		"github.com/jackc/pgx/v5": "v5.7.6",
	}

	tests := []struct {
		name     string
		pkg      string
		expected string
	}{
		{"existing package", "github.com/google/uuid", "v1.6.0"},
		{"another existing package", "github.com/jackc/pgx/v5", "v5.7.6"},
		{"non-existing package", "github.com/unknown/pkg", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetVersion(versions, tt.pkg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGoModParser_ParseMultipleRequireBlocks(t *testing.T) {
	content := `module github.com/jinford/dev-rag

go 1.24.4

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.6
)

require (
	github.com/stretchr/testify v1.11.1
	github.com/openai/openai-go/v3 v3.8.1
)
`

	parser := NewGoModParser()
	versions, err := parser.ParseContent(content)
	require.NoError(t, err)

	// すべてのrequireブロックがパースされることを確認
	assert.Equal(t, "v1.6.0", versions["github.com/google/uuid"])
	assert.Equal(t, "v5.7.6", versions["github.com/jackc/pgx/v5"])
	assert.Equal(t, "v1.11.1", versions["github.com/stretchr/testify"])
	assert.Equal(t, "v3.8.1", versions["github.com/openai/openai-go/v3"])
}

func TestGoModParser_ParseReplace(t *testing.T) {
	// replaceディレクティブは現在サポート外だが、エラーにならないことを確認
	content := `module github.com/jinford/dev-rag

go 1.24.4

require github.com/google/uuid v1.6.0

replace github.com/google/uuid => ../local-uuid
`

	parser := NewGoModParser()
	versions, err := parser.ParseContent(content)
	require.NoError(t, err)

	// replaceは無視され、requireのみがパースされる
	assert.Equal(t, "v1.6.0", versions["github.com/google/uuid"])
}
