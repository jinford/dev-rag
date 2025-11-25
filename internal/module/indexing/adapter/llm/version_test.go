package llm

import (
	"testing"
)

func TestNewPromptVersionRegistry(t *testing.T) {
	registry := NewPromptVersionRegistry()

	if registry == nil {
		t.Fatal("NewPromptVersionRegistry returned nil")
	}

	if len(registry.versions) == 0 {
		t.Error("registry.versions should not be empty")
	}

	// 初期バージョンが設定されているか確認
	expectedTypes := []PromptType{
		PromptTypeFileSummary,
		PromptTypeChunkSummary,
		PromptTypeDomainClassification,
		PromptTypeActionGeneration,
	}

	for _, promptType := range expectedTypes {
		version, err := registry.GetVersion(promptType)
		if err != nil {
			t.Errorf("GetVersion(%s) returned error: %v", promptType, err)
		}
		if version == "" {
			t.Errorf("GetVersion(%s) returned empty version", promptType)
		}
	}
}

func TestGetVersion(t *testing.T) {
	registry := NewPromptVersionRegistry()

	tests := []struct {
		name        string
		promptType  PromptType
		expectError bool
	}{
		{
			name:        "file_summary",
			promptType:  PromptTypeFileSummary,
			expectError: false,
		},
		{
			name:        "chunk_summary",
			promptType:  PromptTypeChunkSummary,
			expectError: false,
		},
		{
			name:        "domain_classification",
			promptType:  PromptTypeDomainClassification,
			expectError: false,
		},
		{
			name:        "action_generation",
			promptType:  PromptTypeActionGeneration,
			expectError: false,
		},
		{
			name:        "unknown_type",
			promptType:  PromptType("unknown"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := registry.GetVersion(tt.promptType)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if version == "" {
					t.Error("version should not be empty")
				}
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	registry := NewPromptVersionRegistry()

	tests := []struct {
		name            string
		promptType      PromptType
		receivedVersion string
		expectValid     bool
	}{
		{
			name:            "matching_version",
			promptType:      PromptTypeFileSummary,
			receivedVersion: "1.1",
			expectValid:     true,
		},
		{
			name:            "mismatching_version",
			promptType:      PromptTypeFileSummary,
			receivedVersion: "1.0",
			expectValid:     false,
		},
		{
			name:            "different_major_version",
			promptType:      PromptTypeFileSummary,
			receivedVersion: "2.0",
			expectValid:     false,
		},
		{
			name:            "empty_version",
			promptType:      PromptTypeFileSummary,
			receivedVersion: "",
			expectValid:     false,
		},
		{
			name:            "unknown_prompt_type",
			promptType:      PromptType("unknown"),
			receivedVersion: "1.1",
			expectValid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := registry.ValidateVersion(tt.promptType, tt.receivedVersion)

			if valid != tt.expectValid {
				t.Errorf("ValidateVersion() = %v, want %v", valid, tt.expectValid)
			}
		})
	}
}

func TestUpdateVersion(t *testing.T) {
	registry := NewPromptVersionRegistry()

	tests := []struct {
		name        string
		promptType  PromptType
		newVersion  string
		expectError bool
	}{
		{
			name:        "update_file_summary",
			promptType:  PromptTypeFileSummary,
			newVersion:  "1.2",
			expectError: false,
		},
		{
			name:        "update_chunk_summary",
			promptType:  PromptTypeChunkSummary,
			newVersion:  "2.0",
			expectError: false,
		},
		{
			name:        "unknown_prompt_type",
			promptType:  PromptType("unknown"),
			newVersion:  "1.0",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.UpdateVersion(tt.promptType, tt.newVersion)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// 更新されたバージョンを確認
				version, err := registry.GetVersion(tt.promptType)
				if err != nil {
					t.Errorf("GetVersion() returned error: %v", err)
				}
				if version != tt.newVersion {
					t.Errorf("version = %s, want %s", version, tt.newVersion)
				}
			}
		})
	}
}

func TestIsCompatible(t *testing.T) {
	registry := NewPromptVersionRegistry()

	tests := []struct {
		name        string
		promptType  PromptType
		version     string
		expectCompat bool
	}{
		{
			name:        "same_version",
			promptType:  PromptTypeFileSummary,
			version:     "1.1",
			expectCompat: true,
		},
		{
			name:        "same_major_different_minor",
			promptType:  PromptTypeFileSummary,
			version:     "1.0",
			expectCompat: true,
		},
		{
			name:        "different_major_version",
			promptType:  PromptTypeFileSummary,
			version:     "2.0",
			expectCompat: false,
		},
		{
			name:        "empty_version",
			promptType:  PromptTypeFileSummary,
			version:     "",
			expectCompat: false,
		},
		{
			name:        "unknown_prompt_type",
			promptType:  PromptType("unknown"),
			version:     "1.1",
			expectCompat: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compatible := registry.IsCompatible(tt.promptType, tt.version)

			if compatible != tt.expectCompat {
				t.Errorf("IsCompatible() = %v, want %v", compatible, tt.expectCompat)
			}
		})
	}
}

func TestGetAllVersions(t *testing.T) {
	registry := NewPromptVersionRegistry()

	allVersions := registry.GetAllVersions()

	if len(allVersions) == 0 {
		t.Error("GetAllVersions() returned empty map")
	}

	// すべての期待されるプロンプトタイプが含まれているか確認
	expectedTypes := []PromptType{
		PromptTypeFileSummary,
		PromptTypeChunkSummary,
		PromptTypeDomainClassification,
		PromptTypeActionGeneration,
	}

	for _, promptType := range expectedTypes {
		if _, ok := allVersions[promptType]; !ok {
			t.Errorf("GetAllVersions() missing prompt type: %s", promptType)
		}
	}

	// 返されたマップを変更しても内部状態に影響しないことを確認
	allVersions[PromptTypeFileSummary] = "99.99"

	version, err := registry.GetVersion(PromptTypeFileSummary)
	if err != nil {
		t.Errorf("GetVersion() returned error: %v", err)
	}
	if version == "99.99" {
		t.Error("GetAllVersions() should return a copy, not the internal map")
	}
}

func TestDefaultPromptVersionRegistry(t *testing.T) {
	if DefaultPromptVersionRegistry == nil {
		t.Fatal("DefaultPromptVersionRegistry is nil")
	}

	version, err := DefaultPromptVersionRegistry.GetVersion(PromptTypeFileSummary)
	if err != nil {
		t.Errorf("GetVersion() returned error: %v", err)
	}
	if version == "" {
		t.Error("version should not be empty")
	}
}

func TestVersionCompatibilityScenarios(t *testing.T) {
	registry := NewPromptVersionRegistry()

	// バージョン1.1を期待
	t.Run("backward_compatibility", func(t *testing.T) {
		// 古いバージョン1.0の応答を受信
		compatible := registry.IsCompatible(PromptTypeFileSummary, "1.0")
		if !compatible {
			t.Error("1.0 should be compatible with 1.1 (same major version)")
		}
	})

	t.Run("forward_compatibility", func(t *testing.T) {
		// 新しいバージョン1.2の応答を受信
		compatible := registry.IsCompatible(PromptTypeFileSummary, "1.2")
		if !compatible {
			t.Error("1.2 should be compatible with 1.1 (same major version)")
		}
	})

	t.Run("major_version_incompatibility", func(t *testing.T) {
		// メジャーバージョンが異なる2.0の応答を受信
		compatible := registry.IsCompatible(PromptTypeFileSummary, "2.0")
		if compatible {
			t.Error("2.0 should not be compatible with 1.1 (different major version)")
		}
	})
}

func TestVersionUpdateWorkflow(t *testing.T) {
	registry := NewPromptVersionRegistry()

	// 初期バージョンを確認
	initialVersion, err := registry.GetVersion(PromptTypeFileSummary)
	if err != nil {
		t.Fatalf("GetVersion() returned error: %v", err)
	}

	// バージョンを更新
	newVersion := "1.2"
	err = registry.UpdateVersion(PromptTypeFileSummary, newVersion)
	if err != nil {
		t.Fatalf("UpdateVersion() returned error: %v", err)
	}

	// 更新後のバージョンを確認
	updatedVersion, err := registry.GetVersion(PromptTypeFileSummary)
	if err != nil {
		t.Fatalf("GetVersion() returned error: %v", err)
	}
	if updatedVersion != newVersion {
		t.Errorf("version = %s, want %s", updatedVersion, newVersion)
	}

	// 古いバージョンとの互換性を確認
	compatible := registry.IsCompatible(PromptTypeFileSummary, initialVersion)
	if !compatible {
		t.Errorf("old version %s should be compatible with new version %s", initialVersion, newVersion)
	}

	// 検証が失敗することを確認
	valid := registry.ValidateVersion(PromptTypeFileSummary, initialVersion)
	if valid {
		t.Error("ValidateVersion() should return false for old version")
	}

	// 新しいバージョンでの検証が成功することを確認
	valid = registry.ValidateVersion(PromptTypeFileSummary, newVersion)
	if !valid {
		t.Error("ValidateVersion() should return true for new version")
	}
}
