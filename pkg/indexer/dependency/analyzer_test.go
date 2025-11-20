package dependency

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzer_ExtractImports(t *testing.T) {
	source := `package main

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jinford/dev-rag/pkg/models"
)

func main() {
	fmt.Println("Hello")
}
`

	analyzer := NewAnalyzer()
	goModData := map[string]string{
		"github.com/google/uuid":            "v1.6.0",
		"github.com/jinford/dev-rag/pkg/models": "v0.1.0",
	}

	info, err := analyzer.Analyze(source, goModData)
	require.NoError(t, err)
	require.NotNil(t, info)

	// インポート数の確認
	assert.Equal(t, 4, len(info.Imports))

	// 標準ライブラリの確認
	assert.Equal(t, ImportTypeStandard, info.Imports["fmt"].Type)
	assert.Equal(t, ImportTypeStandard, info.Imports["strings"].Type)

	// 外部依存の確認
	assert.Equal(t, ImportTypeExternal, info.Imports["github.com/google/uuid"].Type)
	assert.Equal(t, "v1.6.0", info.Imports["github.com/google/uuid"].Version)

	// 内部パッケージの確認
	assert.Equal(t, ImportTypeInternal, info.Imports["github.com/jinford/dev-rag/pkg/models"].Type)
}

func TestAnalyzer_ExtractFunctionCalls(t *testing.T) {
	source := `package main

import (
	"fmt"
	"github.com/google/uuid"
)

func processData(data string) string {
	id := uuid.New()
	fmt.Printf("Processing: %s, ID: %s\n", data, id.String())
	result := transform(data)
	return result
}

func transform(s string) string {
	return s
}
`

	analyzer := NewAnalyzer()
	info, err := analyzer.Analyze(source, nil)
	require.NoError(t, err)

	// 関数呼び出しの確認
	assert.Greater(t, len(info.FunctionCalls), 0)

	// uuid.New() の呼び出しを探す
	var foundUUIDNew bool
	for _, call := range info.FunctionCalls {
		if call.Name == "New" && call.Package == "uuid" {
			foundUUIDNew = true
			assert.Equal(t, CallTypeExternal, call.Type)
		}
	}
	assert.True(t, foundUUIDNew, "uuid.New() call should be found")

	// 内部関数呼び出しを探す
	var foundTransform bool
	for _, call := range info.FunctionCalls {
		if call.Name == "transform" {
			foundTransform = true
			assert.Equal(t, CallTypeInternal, call.Type)
		}
	}
	assert.True(t, foundTransform, "transform() call should be found")
}

func TestAnalyzer_ExtractTypeDependencies(t *testing.T) {
	source := `package main

type User struct {
	ID   string
	Name string
	Age  int
}

type Repository interface {
	GetUser(id string) (*User, error)
	SaveUser(user *User) error
}

func CreateUser(name string, age int) *User {
	return &User{
		Name: name,
		Age:  age,
	}
}
`

	analyzer := NewAnalyzer()
	info, err := analyzer.Analyze(source, nil)
	require.NoError(t, err)

	// 型定義の確認
	assert.Contains(t, info.TypeDeps, "User")
	assert.Contains(t, info.TypeDeps, "Repository")

	// User構造体の確認
	userType := info.TypeDeps["User"]
	assert.Equal(t, TypeKindStruct, userType.Kind)
	assert.Equal(t, 3, len(userType.FieldTypes))

	// Repositoryインターフェースの確認
	repoType := info.TypeDeps["Repository"]
	assert.Equal(t, TypeKindInterface, repoType.Kind)
}

func TestAnalyzer_MethodCalls(t *testing.T) {
	source := `package main

type Calculator struct {
	value int
}

func (c *Calculator) Add(x int) {
	c.value += x
}

func (c *Calculator) Result() int {
	return c.value
}

func main() {
	calc := &Calculator{}
	calc.Add(5)
	result := calc.Result()
	_ = result
}
`

	analyzer := NewAnalyzer()
	info, err := analyzer.Analyze(source, nil)
	require.NoError(t, err)

	// メソッド呼び出しの確認
	var foundAdd bool
	var foundResult bool

	for _, call := range info.FunctionCalls {
		if call.Name == "Add" {
			foundAdd = true
			// メソッド呼び出しが検出されることを確認（CallTypeMethodまたはCallTypeExternal）
			assert.True(t, call.Type == CallTypeMethod || call.Type == CallTypeExternal)
		}
		if call.Name == "Result" {
			foundResult = true
			assert.True(t, call.Type == CallTypeMethod || call.Type == CallTypeExternal)
		}
	}

	assert.True(t, foundAdd, "Add() method call should be found")
	assert.True(t, foundResult, "Result() method call should be found")
}

func TestAnalyzer_ComplexTypes(t *testing.T) {
	source := `package main

type StringMap map[string]string

type UserList []User

type User struct {
	ID   string
	Tags []string
	Meta map[string]interface{}
}

func ProcessUsers(users []User) map[string]User {
	result := make(map[string]User)
	for _, user := range users {
		result[user.ID] = user
	}
	return result
}
`

	analyzer := NewAnalyzer()
	info, err := analyzer.Analyze(source, nil)
	require.NoError(t, err)

	// 型定義の確認
	assert.Contains(t, info.TypeDeps, "StringMap")
	assert.Contains(t, info.TypeDeps, "UserList")
	assert.Contains(t, info.TypeDeps, "User")

	// User構造体のフィールド型を確認
	userType := info.TypeDeps["User"]
	assert.Contains(t, userType.FieldTypes, "string")
	assert.Contains(t, userType.FieldTypes, "[]string")
}

func TestAnalyzer_ImportUsage(t *testing.T) {
	source := `package main

import (
	"fmt"
	"strings"
	"unused"

	"github.com/google/uuid"
)

func main() {
	id := uuid.New()
	msg := fmt.Sprintf("ID: %s", id.String())
	fmt.Println(msg)
}
`

	analyzer := NewAnalyzer()
	goModData := map[string]string{
		"github.com/google/uuid": "v1.6.0",
	}

	info, err := analyzer.Analyze(source, goModData)
	require.NoError(t, err)

	// 使用されているインポート
	assert.True(t, info.Imports["fmt"].IsUsed)
	assert.Greater(t, info.Imports["fmt"].UsageCount, 0)

	assert.True(t, info.Imports["github.com/google/uuid"].IsUsed)
	assert.Greater(t, info.Imports["github.com/google/uuid"].UsageCount, 0)

	// 使用されていないインポート
	assert.False(t, info.Imports["strings"].IsUsed)
	assert.False(t, info.Imports["unused"].IsUsed)
}

func TestAnalyzer_FunctionParameters(t *testing.T) {
	source := `package main

type User struct {
	Name string
}

func ProcessUser(user *User, options map[string]interface{}) error {
	return nil
}
`

	analyzer := NewAnalyzer()
	info, err := analyzer.Analyze(source, nil)
	require.NoError(t, err)

	// 型依存の確認
	// *Userとして記録されるため、ポインタ型を確認
	userPtrType, hasPtrType := info.TypeDeps["*User"]
	userType, hasUserType := info.TypeDeps["User"]

	// いずれかが存在し、ProcessUserをパラメータとして使用していることを確認
	assert.True(t, hasPtrType || hasUserType, "User or *User type should exist")

	if hasPtrType {
		assert.Contains(t, userPtrType.ParameterOf, "ProcessUser")
	}
	if hasUserType {
		// Userが定義されている場合はstructとして登録される
		assert.Equal(t, TypeKindStruct, userType.Kind)
	}
}

func TestAnalyzer_BuiltinFunctions(t *testing.T) {
	source := `package main

func main() {
	slice := make([]int, 0, 10)
	slice = append(slice, 1, 2, 3)
	length := len(slice)
	_ = length
}
`

	analyzer := NewAnalyzer()
	info, err := analyzer.Analyze(source, nil)
	require.NoError(t, err)

	// ビルトイン関数の確認
	var foundMake, foundAppend, foundLen bool

	for _, call := range info.FunctionCalls {
		switch call.Name {
		case "make":
			foundMake = true
			assert.Equal(t, CallTypeBuiltin, call.Type)
		case "append":
			foundAppend = true
			assert.Equal(t, CallTypeBuiltin, call.Type)
		case "len":
			foundLen = true
			assert.Equal(t, CallTypeBuiltin, call.Type)
		}
	}

	assert.True(t, foundMake, "make() should be found")
	assert.True(t, foundAppend, "append() should be found")
	assert.True(t, foundLen, "len() should be found")
}
