# dev-rag

社内リポジトリ向け RAG 基盤および Wiki 自動生成システム

## 概要

dev-ragは、Gitリポジトリのコードとドキュメントをインデックス化し、ベクトル検索を可能にするRAG基盤システムです。
インデックスを活用して、リポジトリの技術Wikiを自動生成する機能も提供します。

### 主な機能

- **インデックス化**: Gitリポジトリをクローンし、ファイルをチャンク化してEmbeddingベクトルを生成
- **ベクトル検索**: PostgreSQL + pgvectorを使った意味検索
- **Wiki自動生成**: インデックスを元にMarkdown形式のWikiを生成（Mermaid図を含む）
- **REST API**: インデックス更新やWiki生成をトリガーするHTTPエンドポイント
- **差分更新**: 変更されたファイルのみを再インデックス

## 技術スタック

- **言語**: Go 1.25
- **データベース**: PostgreSQL 16 + pgvector
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

### 5. アプリケーションのビルド（予定）

```bash
# 依存関係のインストール
go mod download

# ビルド
go build -o bin/dev-rag ./cmd/dev-rag

# 実行例
./bin/dev-rag --help
```

## 使い方（予定）

### インデックス作成

```bash
dev-rag index --url git@github.com:company/myapp.git --ref main
```

### Wiki生成

```bash
dev-rag wiki generate --repo myapp
```

### HTTPサーバ起動

```bash
dev-rag server start
```

## ドキュメント

詳細な設計やAPI仕様は以下を参照してください：

- [要件定義書](docs/requirements.md)
- [設計書](docs/design.md)
- [データベーススキーマ](docs/database-schema.md)
- [API/インターフェース](docs/api-interface.md)
- [バックエンドライブラリ](docs/backend-libraries.md)

## ディレクトリ構成

```
dev-rag/
├── cmd/
│   └── dev-rag/         # CLIエントリポイント（予定）
├── pkg/
│   ├── config/          # 設定管理（予定）
│   ├── db/              # DB接続（予定）
│   ├── models/          # データモデル（予定）
│   ├── repository/      # リポジトリ管理（予定）
│   ├── indexer/         # インデックス処理（予定）
│   ├── search/          # ベクトル検索（予定）
│   ├── wiki/            # Wiki生成（予定）
│   ├── server/          # HTTPサーバ（予定）
│   └── jobs/            # ジョブ管理（予定）
├── schema/              # DBスキーマ定義
│   └── schema.sql
├── docs/                # ドキュメント
├── compose.yaml         # 開発用DB
├── .env.example         # 環境変数サンプル
└── README.md
```

## 開発

### データベースのリセット

```bash
# コンテナとボリュームを削除
docker compose down -v

# 再起動（スキーマが再初期化される）
docker compose up -d
```

### ログの確認

```bash
docker compose logs -f postgres
```