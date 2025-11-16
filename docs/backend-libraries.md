# バックエンド実装に必要なライブラリ

**システム名：社内リポジトリ向け RAG 基盤および Wiki 自動生成システム**

---

## 1. 概要

本ドキュメントは、dev-ragシステムのバックエンド実装に必要なGoライブラリを整理したものです。
設計書および要件定義書に基づいて、各機能領域ごとに推奨ライブラリをリストアップしています。

---

## 2. コア機能別ライブラリ

### 2.1 データベース関連

#### PostgreSQL接続・クエリ

- **pgx** (github.com/jackc/pgx/v5)
  - 用途: PostgreSQL公式のGoドライバ
  - 理由: 高性能、pgvectorとの相性が良い、標準的なdatabase/sqlインターフェース対応
  - 主な使用箇所: DB接続、トランザクション管理、クエリ実行

#### pgvectorサポート

- **pgvector-go** (github.com/pgvector/pgvector-go)
  - 用途: pgvectorのベクトル型をGoで扱うためのライブラリ
  - 理由: pgxと連携してVECTOR型のエンコード/デコードが可能
  - 主な使用箇所: Embeddingベクトルの保存・検索

#### スキーマ初期化

- **シンプルなSQLスクリプトで対応**
  - 用途: データベーススキーマの初期化
  - 理由: プロジェクト規模が小さく、マイグレーションライブラリは過剰
  - 主な使用箇所: migrations/init.sql を直接実行
  - 実装例: `psql` コマンドまたは pgx でSQLファイルを読み込んで実行

---

### 2.2 Git操作

#### Git操作ライブラリ

- **go-git** (github.com/go-git/go-git/v5)
  - 用途: Gitリポジトリのクローン、pull、ファイル一覧取得
  - 理由: Pure Goで実装されており、外部のgitコマンド不要、SSH認証サポート
  - 主な使用箇所: インデックス処理（リポジトリクローン、更新）

**代替案:** exec.Commandでgitコマンドを直接実行（シンプルだが環境依存）

---

### 2.3 外部API連携

#### OpenAI API（Embeddings、LLM生成）

- **openai-go** (github.com/openai/openai-go/v3)
  - 用途: OpenAI Embeddings API、Chat Completions API呼び出し
  - 理由: 公式SDKで安定性が高く、最新のAPI機能に対応
  - 主な使用箇所: Embedding生成、Wiki生成（OpenAIモデル使用時）

#### Anthropic Claude API

- **anthropic-sdk-go** (github.com/anthropics/anthropic-sdk-go)
  - 用途: Claude API呼び出し（Wiki生成用LLM）
  - 理由: 公式SDK、メッセージAPI対応
  - 主な使用箇所: Wiki生成（Claudeモデル使用時）

---

### 2.4 HTTP サーバ・REST API

#### Webフレームワーク

- **Echo** (github.com/labstack/echo/v4)
  - 用途: HTTPサーバ、REST APIエンドポイント、ミドルウェア
  - 理由: 軽量で高速、ミドルウェアが豊富、認証機能サポート
  - 主な使用箇所: HTTP サーバ、APIハンドラ、認証ミドルウェア

**代替案:**
- Gin (github.com/gin-gonic/gin) - より高速だが機能はEchoと同等
- net/http（標準ライブラリ）- フレームワーク不要の場合

#### Bearer Token認証

- **echo-jwt** (github.com/labstack/echo-jwt/v4)
  - 用途: JWT/Bearer Token認証ミドルウェア
  - 理由: Echoと統合されており簡単に導入可能
  - 主な使用箇所: REST API認証

**補足:** シンプルなBearer Token認証なら自前実装でも十分

---

### 2.5 設定管理

#### 環境変数・設定ファイル読み込み

- **godotenv** (github.com/joho/godotenv)
  - 用途: .envファイルからの環境変数読み込み
  - 理由: シンプルで軽量、.env形式の設定ファイルに特化
  - 主な使用箇所: .envファイルの読み込み、環境変数の管理

---

### 2.6 CLI

#### コマンドライン引数・サブコマンド

- **cli** (github.com/urfave/cli/v3)
  - 用途: CLIインターフェース、サブコマンド（index, wiki, server）
  - 理由: シンプルで直感的なAPI、軽量かつ柔軟なCLI構築が可能
  - 主な使用箇所: `dev-rag index`, `dev-rag wiki generate`, `dev-rag server start`

---

### 2.7 ロギング

#### 構造化ログ

- **log/slog** (標準ライブラリ、Go 1.21+)
  - 用途: JSON構造化ログ出力
  - 理由: 標準ライブラリで十分、外部依存なし
  - 主な使用箇所: 全体のログ出力

**注:** Go 1.25を使用するため、log/slogが標準で利用可能

---

### 2.8 チャンク化・トークン推定

#### テキスト分割

- **自前実装を推奨**
  - 理由: 要件に合わせた細かい制御が必要（開始行・終了行、オーバーラップ管理）
  - 主な使用箇所: pkg/indexer/chunker

#### トークン推定

- **tiktoken-go** (github.com/pkoukk/tiktoken-go)
  - 用途: OpenAIトークナイザの移植版、トークン数の正確な推定
  - 理由: OpenAIモデルと互換性のあるトークンカウント
  - 主な使用箇所: チャンク化時のトークン数推定

**代替案:** 簡易実装（文字数 ÷ 4 で推定） - 高精度が不要な場合

---

### 2.9 ファイル種別判定

#### 言語検出

- **go-enry** (github.com/go-enry/go-enry/v2)
  - 用途: ファイルパスから言語種別を判定
  - 理由: GitHub Linguistの移植版、多言語対応
  - 主な使用箇所: ファイル登録時の言語種別判定

---

### 2.10 除外ルール処理

#### .gitignore / .devragignore パーサー

- **gitignore** (github.com/sabhiram/go-gitignore)
  - 用途: .gitignore形式のパターンマッチング
  - 理由: .gitignoreの仕様に準拠、ワイルドカード対応
  - 主な使用箇所: ファイル一覧取得時の除外処理

---

### 2.11 非同期処理・ジョブ管理

#### バックグラウンドジョブ

- **自前実装（Goルーチン + sync.Map）**
  - 理由: 要件がシンプル（メモリ内管理、24時間TTL）
  - 主な使用箇所: pkg/jobs

**代替案（将来的な拡張時）:**
- asynq (github.com/hibiken/asynq) - Redis ベースのジョブキュー
- machinery (github.com/RichardKnop/machinery) - 分散ジョブキュー

---

### 2.12 ハッシュ計算

#### SHA-256ハッシュ

- **crypto/sha256** (標準ライブラリ)
  - 用途: ファイル・チャンク内容のハッシュ計算
  - 理由: 標準ライブラリで十分
  - 主な使用箇所: 差分インデックス時の変更検出

---

### 2.13 UUID生成

#### UUID

- **google/uuid** (github.com/google/uuid)
  - 用途: UUIDの生成
  - 理由: 標準的なライブラリ、PostgreSQLのgen_random_uuid()との互換性
  - 主な使用箇所: エンティティIDの生成（PostgreSQL側で生成する場合は不要）

---

### 2.14 同時実行制御

#### PostgreSQLアドバイザリロック

- **pgx** のアドバイザリロック機能を使用
  - 用途: 同一リポジトリ・参照に対するインデックス処理の排他制御
  - 理由: PostgreSQL組み込み機能、分散ロック不要
  - 主な使用箇所: pkg/indexer

```go
// 例
tx.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID)
defer tx.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID)
```

---

## 3. テスト関連ライブラリ

### 3.1 テストフレームワーク

- **testing** (標準ライブラリ)
  - 用途: 単体テスト
  - 理由: 標準で十分

- **testify** (github.com/stretchr/testify)
  - 用途: アサーション、モック
  - 理由: よりリッチなテスト記述が可能
  - 主な使用箇所: assert, require, mock

### 3.2 テスト用PostgreSQL

- **dockertest** (github.com/ory/dockertest/v3)
  - 用途: Docker上でテスト用PostgreSQLを起動
  - 理由: 統合テストでの一時的なDB環境構築
  - 主な使用箇所: 統合テスト

---

## 4. 推奨ライブラリ一覧（まとめ）

| カテゴリ | ライブラリ | 用途 |
|---------|----------|------|
| **DB接続** | github.com/jackc/pgx/v5 | PostgreSQL接続・クエリ |
| **pgvector** | github.com/pgvector/pgvector-go | ベクトル型サポート |
| **スキーマ初期化** | SQLスクリプト | init.sqlを直接実行 |
| **Git操作** | github.com/go-git/go-git/v5 | リポジトリクローン・更新 |
| **OpenAI API** | github.com/openai/openai-go/v3 | Embeddings、Chat Completions |
| **Anthropic API** | github.com/anthropics/anthropic-sdk-go | Claude API |
| **Webフレームワーク** | github.com/labstack/echo/v4 | HTTP サーバ、REST API |
| **認証** | github.com/labstack/echo-jwt/v4 | Bearer Token認証 |
| **設定管理** | github.com/joho/godotenv | .env環境変数 |
| **CLI** | github.com/urfave/cli/v3 | サブコマンド |
| **ログ** | log/slog（標準） | 構造化ログ |
| **トークン推定** | github.com/pkoukk/tiktoken-go | トークン数カウント |
| **言語検出** | github.com/go-enry/go-enry/v2 | ファイル種別判定 |
| **除外ルール** | github.com/sabhiram/go-gitignore | .gitignoreパース |
| **UUID** | github.com/google/uuid | UUID生成 |
| **テスト** | github.com/stretchr/testify | アサーション・モック |
| **テスト用DB** | github.com/ory/dockertest/v3 | Docker上のPostgreSQL |

---

## 5. go.mod 初期設定例

```go
module github.com/yourusername/dev-rag

go 1.25

require (
	github.com/anthropics/anthropic-sdk-go v0.1.0
	github.com/go-enry/go-enry/v2 v2.8.8
	github.com/go-git/go-git/v5 v5.12.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.5.5
	github.com/joho/godotenv v1.5.1
	github.com/labstack/echo/v4 v4.12.0
	github.com/labstack/echo-jwt/v4 v4.2.0
	github.com/openai/openai-go/v3 v3.0.0
	github.com/ory/dockertest/v3 v3.10.0
	github.com/pgvector/pgvector-go v0.1.1
	github.com/pkoukk/tiktoken-go v0.1.6
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06
	github.com/stretchr/testify v1.9.0
	github.com/urfave/cli/v3 v3.0.0-alpha9
)
```

---

## 6. 実装時の注意点

### 6.1 依存関係の最小化

- 標準ライブラリで十分な場合は外部ライブラリを避ける
- 小規模な機能（UUIDなど）はPostgreSQL側で生成することも検討

### 6.2 バージョン管理

- `go.mod` で明示的にバージョンを固定
- 定期的に `go get -u` で更新し、セキュリティパッチを適用

### 6.3 ライセンス確認

- 各ライブラリのライセンスを確認し、社内利用規定に準拠すること
- 主なライセンス:
  - MIT: pgx, echo, cli, godotenv（商用利用可）
  - Apache 2.0: anthropic-sdk-go, openai-go（商用利用可）
  - BSD: go-git（商用利用可）

---

## 7. まとめ

本ドキュメントでは、dev-ragバックエンドの実装に必要な主要ライブラリを整理しました。

**推奨アプローチ:**
1. まず標準ライブラリを優先
2. 実績のある安定したライブラリを選択
3. 過度な抽象化を避け、シンプルな実装を心がける

**次のステップ:**
1. `go mod init` でプロジェクト初期化
2. 必要なライブラリを `go get` でインストール
3. 各パッケージの実装開始
