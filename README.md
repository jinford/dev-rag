# dev-rag

マルチソース対応 RAG 基盤および Wiki 自動生成システム

## 概要

dev-ragは、複数の情報ソース（Git、Confluence、PDF等）のコードとドキュメントをインデックス化し、ベクトル検索を可能にするRAG基盤システムです。
プロダクト単位で複数のソースを統合し、技術Wikiを自動生成する機能も提供します。

### 主な機能

- **マルチソース対応**: Git、Confluence、PDF、Redmine、Notion、ローカルファイルを統合管理（初期フェーズはGitのみ実装）
- **プロダクト管理**: 複数のソースをプロダクト単位でグループ化
- **インデックス化**: 情報ソースをクローンし、ファイルをチャンク化してEmbeddingベクトルを生成
- **ベクトル検索**: PostgreSQL + pgvectorを使った意味検索（プロダクト横断検索に対応）
- **Wiki自動生成**: プロダクト単位でMarkdown形式のWikiを生成（Mermaid図を含む）
- **REST API**: インデックス更新やWiki生成をトリガーするHTTPエンドポイント
- **差分更新**: 変更されたファイルのみを再インデックス
- **Git参照管理**: ブランチ、タグごとのスナップショット管理

## 技術スタック

- **言語**: Go 1.25
- **データベース**: PostgreSQL 18 + pgvector
- **外部API**: OpenAI (Embeddings, LLM), Anthropic Claude (LLM)
- **Webフレームワーク**: Echo v4
- **CLI**: urfave/cli/v3

## セットアップ

### 前提条件

- Docker & Docker Compose
- Go 1.25以上

### 1. リポジトリのクローン

```bash
git clone <repository-url>
cd dev-rag
```

### 2. 環境変数の設定

```bash
cp .env.example .env
# .envファイルを編集してAPIキーなどを設定
```

### 3. データベースの起動

```bash
docker compose up -d
```

PostgreSQL + pgvectorが起動し、自動的にスキーマが初期化されます。

### 4. データベースの確認

```bash
# PostgreSQLに接続
docker compose exec postgres psql -U devrag -d devrag

# テーブル一覧を確認
\dt

# pgvector拡張を確認
\dx
```

### 5. アプリケーションのビルド

#### Makefileを使用する場合（推奨）

```bash
# 開発環境のセットアップ（.envファイル作成、依存関係インストール）
make dev-setup

# ビルド
make build

# ヘルプを表示
make help
```

#### 手動でビルドする場合

```bash
# 依存関係のインストール
go mod download

# ビルド
go build -o bin/dev-rag ./cmd/dev-rag

# 実行例
./bin/dev-rag --help
```

## 使い方

### Makefileコマンド一覧

```bash
# ビルド関連
make build        # バイナリをビルド
make clean        # ビルド成果物を削除
make rebuild      # クリーン→ビルド
make install      # 依存関係をインストール
make dev-setup    # 開発環境のセットアップ

# データベース関連
make db-up        # データベースを起動
make db-down      # データベースを停止
make db-reset     # データベースをリセット
make db-logs      # データベースのログ確認

# テスト関連
make test         # テストを実行
make test-coverage # カバレッジ付きテスト

# コード品質
make fmt          # コードフォーマット
make lint         # リンター実行

# プロダクト管理
make run-product CMD=list                    # プロダクト一覧
make run-product CMD=show NAME=ecommerce     # プロダクト詳細

# ソース管理
make run-source CMD=list                     # ソース一覧
make run-source CMD=list PRODUCT=ecommerce   # プロダクト配下のソース一覧
make run-source CMD=show NAME=backend-api    # ソース詳細

# インデックス作成（Gitソース）
make run-index URL=git@github.com:user/backend.git NAME=backend-api PRODUCT=ecommerce
make run-index URL=git@github.com:user/backend.git NAME=backend-api PRODUCT=ecommerce REF=main
make run-index URL=git@github.com:user/backend.git NAME=backend-api PRODUCT=ecommerce FORCE_INIT=true

# Wiki生成（プロダクト単位）
make run-wiki PRODUCT=ecommerce              # デフォルト出力先
make run-wiki PRODUCT=ecommerce OUT=/custom/path  # カスタム出力先

# HTTPサーバ起動
make run-server                              # デフォルトポート8080
make run-server PORT=9000                    # カスタムポート
```

### CLIコマンド例

#### プロダクト管理

```bash
# プロダクト一覧
./bin/dev-rag product list

# プロダクト詳細
./bin/dev-rag product show --name ecommerce
```

#### ソース管理とインデックス作成

```bash
# Gitソースのインデックス作成（プロダクト自動作成、ソース自動登録）
# Makefileを使用
make run-index URL=git@github.com:company/backend.git NAME=backend-api PRODUCT=ecommerce REF=main

# 直接実行（--name を省略すると Git URL から自動で決定）
./bin/dev-rag index git \
  --url git@github.com:company/backend.git \
  --name backend-api \
  --product ecommerce \
  --ref main

# 複数のソースを同じプロダクトに登録
./bin/dev-rag index git \
  --url git@github.com:company/frontend.git \
  --name frontend-web \
  --product ecommerce

./bin/dev-rag index git \
  --url git@github.com:company/infra.git \
  --name infra \
  --product ecommerce

# ソース一覧
./bin/dev-rag source list
./bin/dev-rag source list --product ecommerce

# ソース詳細
./bin/dev-rag source show --name backend-api

# 強制フルインデックス
./bin/dev-rag index git \
  --url git@github.com:company/backend.git \
  --name backend-api \
  --product ecommerce \
  --force-init
```

#### Wiki生成（プロダクト単位）

```bash
# Makefileを使用
make run-wiki PRODUCT=ecommerce

# 直接実行
./bin/dev-rag wiki generate --product ecommerce

# カスタム出力ディレクトリ
./bin/dev-rag wiki generate --product ecommerce --out /custom/path
```

#### HTTPサーバ起動

```bash
# Makefileを使用
make run-server PORT=8080

# 直接実行
./bin/dev-rag server start
./bin/dev-rag server start --port 8080
```

## ドキュメント

詳細な設計やAPI仕様は以下を参照してください：

- [要件定義書](docs/requirements.md) - システム概要、目的、機能要件、非機能要件
- [設計書](docs/design.md) - システムアーキテクチャ、データモデル、CLI仕様、Wiki生成設計
- [データベーススキーマ](docs/database-schema.md) - テーブル定義、ER図、クエリ例
- [API/インターフェース](docs/api-interface.md) - 内部インターフェース要件、HTTPエンドポイント仕様

## ディレクトリ構成

```
dev-rag/
├── cmd/
│   └── dev-rag/            # CLIエントリポイント
│       └── main.go
├── internal/               # 内部実装（非公開パッケージ）
│   ├── interface/          # インターフェース層
│   │   ├── cli/            # CLIコマンド定義
│   │   │   ├── product.go  # プロダクト管理コマンド
│   │   │   ├── source.go   # ソース管理コマンド
│   │   │   ├── wiki.go     # Wiki生成コマンド
│   │   │   └── server.go   # HTTPサーバコマンド
│   │   └── http/           # HTTPハンドラ（未実装）
│   ├── module/             # ドメインモジュール（境界づけられたコンテキスト）
│   │   ├── indexing/       # インデックス管理モジュール
│   │   │   ├── domain/     # ドメインモデルとインターフェース
│   │   │   ├── application/# アプリケーションサービス
│   │   │   └── adapter/    # インフラストラクチャ実装
│   │   │       └── pg/     # PostgreSQLアダプター（sqlc生成コード含む）
│   │   ├── search/         # 検索モジュール
│   │   │   ├── domain/
│   │   │   ├── application/
│   │   │   └── adapter/pg/
│   │   ├── wiki/           # Wiki生成モジュール
│   │   │   ├── domain/
│   │   │   ├── application/
│   │   │   └── adapter/pg/
│   │   └── llm/            # LLMクライアントモジュール
│   │       ├── domain/     # LLMインターフェース定義
│   │       └── adapter/    # OpenAI/Anthropic実装
│   └── platform/           # 横断的関心事
│       ├── config/         # 設定管理
│       ├── database/       # データベース接続・トランザクション
│       └── container/      # DIコンテナ
├── pkg/                    # 公開可能な汎用パッケージ（レガシー）
│   ├── db/                 # DB接続（互換性のため残存）
│   ├── lock/               # ロック管理（互換性のため残存）
│   ├── repository/         # リポジトリ実装（互換性のため残存）
│   ├── indexer/            # インデックス処理（段階的移行中）
│   │   ├── chunker/        # チャンク化ロジック
│   │   ├── embedder/       # Embedding生成
│   │   └── ...
│   ├── search/             # ベクトル検索（段階的移行中）
│   ├── wiki/               # Wiki生成（段階的移行中）
│   ├── query/              # クエリ処理
│   └── provenance/         # 来歴管理
├── schema/                 # DBスキーマ定義
│   └── schema.sql
├── docs/                   # ドキュメント
│   ├── requirements.md     # 要件定義書
│   ├── design.md           # 設計書
│   ├── database-schema.md  # データベーススキーマ
│   └── api-interface.md    # API/インターフェース
├── wiki-viewer/            # Next.js Webアプリ（未実装）
├── Makefile                # ビルドタスク
├── compose.yaml            # 開発用DB
├── .env.example            # 環境変数サンプル
├── .gitignore
└── README.md
```

### アーキテクチャの特徴

- **レイヤードアーキテクチャ**: Interface → Application → Domain → Adapter の4層構造
- **境界づけられたコンテキスト**: indexing, search, wiki, llm の各モジュールを独立管理
- **依存性逆転**: ドメイン層がアダプター層に依存しない設計
- **段階的移行**: `pkg/` から `internal/module/` へ段階的にリファクタリング中

## 実装状況

### 完了
- ✓ プロジェクト構造（レイヤードアーキテクチャ）
- ✓ Go module初期化とライブラリインストール
- ✓ 設定管理（internal/platform/config）
- ✓ データベース接続・トランザクション（internal/platform/database）
- ✓ DIコンテナ（internal/platform/container）
- ✓ ドメインモデル定義（internal/module/*/domain）
- ✓ リポジトリ実装（internal/module/*/adapter/pg + sqlc）
- ✓ アプリケーションサービス（internal/module/*/application）
- ✓ LLMクライアント（internal/module/llm）
- ✓ CLIエントリポイント（cmd/dev-rag）
- ✓ CLIコマンド（internal/interface/cli: product/source/index/wiki/server）
- ✓ プロダクト管理機能
- ✓ ソース管理機能（Git対応）
- ✓ インデックス処理（pkg/indexer）
  - Gitクローン/pull
  - チャンク化（pkg/indexer/chunker）
  - Embedding生成（pkg/indexer/embedder）
  - 差分更新ロジック
  - ファイルサマリー生成
  - 依存関係グラフ構築
  - 重要度スコア計算
  - カバレッジ分析
- ✓ ベクトル検索（pkg/search）
  - プロダクト横断検索
  - ソース単位検索
  - 階層的検索
- ✓ Wiki生成（pkg/wiki）
  - プロダクト単位生成
  - LLM連携（OpenAI/Anthropic）
  - Mermaid図生成
  - アーキテクチャサマリー生成
  - ディレクトリサマリー生成
- ✓ HTTPサーバ（internal/interface/http: echo v4）
- ✓ Makefile

### 未実装・改善予定
- HTTPサーバAPI拡充
  - REST API実装（検索、Wiki生成トリガー等）
  - 非同期ジョブ処理
- wiki-viewer（Next.js Webアプリ）
- pkg/ から internal/module/ への完全移行
- テストカバレッジの拡充

## 開発

### クイックスタート

```bash
# 1. リポジトリをクローン
git clone <repository-url>
cd dev-rag

# 2. 開発環境をセットアップ
make dev-setup

# 3. .envファイルを編集（APIキーを設定）
vi .env

# 4. データベースを起動
make db-up

# 5. ビルド
make build

# 6. 動作確認
./bin/dev-rag --help
```

### データベース操作

```bash
# データベースを起動
make db-up

# データベースを停止
make db-down

# データベースをリセット（コンテナとボリュームを削除して再起動）
make db-reset

# ログの確認
make db-logs
```

### ビルドとテスト

```bash
# ビルド
make build

# テスト実行
make test

# カバレッジ付きテスト
make test-coverage

# コードフォーマット
make fmt
```
