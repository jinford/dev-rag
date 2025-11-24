package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config はアプリケーション全体の設定を保持します
type Config struct {
	// Database設定
	Database DatabaseConfig

	// API認証
	APIToken string

	// OpenAI設定（Embeddings用）
	OpenAI OpenAIConfig

	// Wiki生成用LLM設定
	WikiLLM WikiLLMConfig

	// Git設定
	Git GitConfig

	// Wiki出力設定
	WikiOutputDir string
}

// DatabaseConfig はデータベース接続設定
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// OpenAIConfig はOpenAI API設定（Embeddings + LLM）
type OpenAIConfig struct {
	APIKey             string
	EmbeddingModel     string
	EmbeddingDimension int
	LLMModel           string // LLMモデル名（ファイル要約生成等に使用）
}

// WikiLLMConfig はWiki生成用LLM設定
type WikiLLMConfig struct {
	Provider    string // "openai" or "anthropic"
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
}

// GitConfig はGit操作設定
type GitConfig struct {
	CloneDir      string
	SSHKeyPath    string
	SSHPassword   string // SSH秘密鍵のパスワード（パスフレーズ）
	SSHKnownHosts string
	DefaultBranch string // デフォルトブランチ名（例: main, master）
}

// Load は環境変数または.envファイルから設定を読み込みます
func Load(envFilePath string) (*Config, error) {
	// .envファイルが存在する場合は読み込む
	if envFilePath != "" {
		if err := godotenv.Load(envFilePath); err != nil {
			// ファイルが存在しない場合はエラーとしない（環境変数のみで動作可能）
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to load .env file: %w", err)
			}
		}
	}

	cfg := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "devrag"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "devrag"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		APIToken: getEnv("DEVRAG_API_TOKEN", ""),
		OpenAI: OpenAIConfig{
			APIKey:             getEnv("OPENAI_API_KEY", ""),
			EmbeddingModel:     getEnv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"),
			EmbeddingDimension: getEnvAsInt("OPENAI_EMBEDDING_DIMENSION", 1536),
			LLMModel:           getEnv("OPENAI_LLM_MODEL", "gpt-4o-mini"), // デフォルトはgpt-4o-mini
		},
		WikiLLM: WikiLLMConfig{
			Provider:    getEnv("WIKI_LLM_PROVIDER", "openai"),
			APIKey:      getEnv("WIKI_LLM_API_KEY", ""),
			Model:       getEnv("WIKI_LLM_MODEL", "gpt-4-turbo-preview"),
			Temperature: getEnvAsFloat("WIKI_LLM_TEMPERATURE", 0.2),
			MaxTokens:   getEnvAsInt("WIKI_LLM_MAX_TOKENS", 2048),
		},
		Git: GitConfig{
			CloneDir:      getEnv("GIT_CLONE_DIR", "/var/lib/dev-rag/repos"),
			SSHKeyPath:    getEnv("GIT_SSH_KEY_PATH", "/etc/dev-rag/ssh/id_rsa"),
			SSHPassword:   getEnv("GIT_SSH_PASSWORD", ""),
			SSHKnownHosts: getEnv("GIT_SSH_KNOWN_HOSTS", "/etc/dev-rag/ssh/known_hosts"),
			DefaultBranch: getEnv("GIT_DEFAULT_BRANCH", "main"),
		},
		WikiOutputDir: getEnv("WIKI_OUTPUT_DIR", "/var/lib/dev-rag/wikis"),
	}

	return cfg, nil
}

// getEnv は環境変数を取得し、存在しない場合はデフォルト値を返します
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt は環境変数を整数として取得します
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsFloat は環境変数を浮動小数点数として取得します
func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return defaultValue
	}
	return value
}
