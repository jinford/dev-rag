package llm

import (
	"fmt"
	"log"
)

// PromptType はプロンプトの種類を表す
type PromptType string

const (
	// PromptTypeFileSummary はファイルサマリー生成プロンプト
	PromptTypeFileSummary PromptType = "file_summary"
	// PromptTypeChunkSummary はチャンク要約生成プロンプト
	PromptTypeChunkSummary PromptType = "chunk_summary"
	// PromptTypeDomainClassification はドメイン分類プロンプト
	PromptTypeDomainClassification PromptType = "domain_classification"
	// PromptTypeActionGeneration はアクション生成プロンプト
	PromptTypeActionGeneration PromptType = "action_generation"
)

// PromptVersion はプロンプトのバージョン情報を管理する
type PromptVersion struct {
	Type    PromptType
	Version string
}

// PromptVersionRegistry はプロンプトバージョンの一元管理を行う
type PromptVersionRegistry struct {
	versions map[PromptType]string
}

// NewPromptVersionRegistry は新しいPromptVersionRegistryを作成する
func NewPromptVersionRegistry() *PromptVersionRegistry {
	return &PromptVersionRegistry{
		versions: map[PromptType]string{
			PromptTypeFileSummary:          "1.1",
			PromptTypeChunkSummary:         "1.1",
			PromptTypeDomainClassification: "1.1",
			PromptTypeActionGeneration:     "1.1",
		},
	}
}

// GetVersion は指定されたプロンプトタイプの現在のバージョンを取得する
func (r *PromptVersionRegistry) GetVersion(promptType PromptType) (string, error) {
	version, ok := r.versions[promptType]
	if !ok {
		return "", fmt.Errorf("unknown prompt type: %s", promptType)
	}
	return version, nil
}

// ValidateVersion は応答のプロンプトバージョンを検証する
// 期待するバージョンと異なる場合はログ警告を出力するが、エラーは返さない
func (r *PromptVersionRegistry) ValidateVersion(promptType PromptType, receivedVersion string) bool {
	expectedVersion, err := r.GetVersion(promptType)
	if err != nil {
		log.Printf("[WARN] Failed to get expected version for prompt type %s: %v", promptType, err)
		return false
	}

	if receivedVersion != expectedVersion {
		log.Printf("[WARN] Prompt version mismatch for %s: expected=%s, received=%s",
			promptType, expectedVersion, receivedVersion)
		return false
	}

	return true
}

// UpdateVersion は指定されたプロンプトタイプのバージョンを更新する
// プロンプトを変更した際にこのメソッドを呼び出してバージョンを上げる
func (r *PromptVersionRegistry) UpdateVersion(promptType PromptType, newVersion string) error {
	if _, ok := r.versions[promptType]; !ok {
		return fmt.Errorf("unknown prompt type: %s", promptType)
	}

	oldVersion := r.versions[promptType]
	r.versions[promptType] = newVersion
	log.Printf("[INFO] Updated prompt version for %s: %s -> %s", promptType, oldVersion, newVersion)

	return nil
}

// IsCompatible は古いバージョンの応答が現在でもサポートされているかをチェックする
// バージョン互換性のルール:
// - メジャーバージョン（最初の数字）が同じなら互換性あり
// - 例: 1.0, 1.1, 1.2 は互換性あり、2.0 は互換性なし
func (r *PromptVersionRegistry) IsCompatible(promptType PromptType, version string) bool {
	expectedVersion, err := r.GetVersion(promptType)
	if err != nil {
		log.Printf("[WARN] Failed to get expected version for compatibility check: %v", err)
		return false
	}

	// 簡易的なメジャーバージョンチェック
	// より厳密なバージョン比較が必要な場合は semver ライブラリを使用する
	if len(version) == 0 || len(expectedVersion) == 0 {
		return false
	}

	// 最初の文字（メジャーバージョン）を比較
	return version[0] == expectedVersion[0]
}

// GetAllVersions はすべてのプロンプトタイプとバージョンのマッピングを返す
func (r *PromptVersionRegistry) GetAllVersions() map[PromptType]string {
	// コピーを返して内部状態を保護
	result := make(map[PromptType]string, len(r.versions))
	for k, v := range r.versions {
		result[k] = v
	}
	return result
}

// DefaultPromptVersionRegistry はデフォルトのプロンプトバージョンレジストリ
var DefaultPromptVersionRegistry = NewPromptVersionRegistry()
