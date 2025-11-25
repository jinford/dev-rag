.PHONY: help build clean test run-product run-source run-index run-wiki run-server install dev-setup db-up db-down db-reset sqlc-generate

# デフォルトターゲット
help:
	@echo "利用可能なコマンド:"
	@echo "  make build        - バイナリをビルド"
	@echo "  make clean        - ビルド成果物を削除"
	@echo "  make test         - テストを実行"
	@echo "  make install      - 依存関係をインストール"
	@echo "  make dev-setup    - 開発環境のセットアップ"
	@echo ""
	@echo "  make db-up        - データベースを起動"
	@echo "  make db-down      - データベースを停止"
	@echo "  make db-reset     - データベースをリセット"
	@echo "  make sqlc-generate - SQLクエリからGoコードを生成"
	@echo ""
	@echo "  make run-product  - プロダクト管理コマンドを実行"
	@echo "  make run-source   - ソース管理コマンドを実行"
	@echo "  make run-index    - インデックスコマンドを実行（Gitソース）"
	@echo "  make run-wiki     - Wiki生成コマンドを実行（プロダクト単位）"
	@echo "  make run-server   - HTTPサーバを起動"

# バイナリのビルド
build:
	@echo "バイナリをビルド中..."
	@mkdir -p bin
	@go build -o bin/dev-rag ./cmd/dev-rag
	@echo "✓ ビルド完了: bin/dev-rag"

# ビルド成果物の削除
clean:
	@echo "ビルド成果物を削除中..."
	@rm -rf bin/
	@go clean
	@echo "✓ クリーンアップ完了"

# 依存関係のインストール
install:
	@echo "依存関係をインストール中..."
	@go mod download
	@go mod tidy
	@echo "✓ 依存関係のインストール完了"

# テストの実行
test:
	@echo "テストを実行中..."
	@go test -v ./...
	@echo "✓ テスト完了"

# テスト（カバレッジ付き）
test-coverage:
	@echo "テスト（カバレッジ付き）を実行中..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ カバレッジレポート: coverage.html"

# 開発環境のセットアップ
dev-setup: install
	@echo "開発環境をセットアップ中..."
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "✓ .env ファイルを作成しました。必要に応じて編集してください。"; \
	else \
		echo "✓ .env ファイルは既に存在します"; \
	fi
	@mkdir -p /tmp/dev-rag/repos /tmp/dev-rag/wikis
	@echo "✓ 開発用ディレクトリを作成しました"

# データベースの起動
db-up:
	@echo "データベースを起動中..."
	@docker compose up -d
	@echo "✓ データベースが起動しました"

# データベースの停止
db-down:
	@echo "データベースを停止中..."
	@docker compose down
	@echo "✓ データベースを停止しました"

# データベースのリセット
db-reset:
	@echo "データベースをリセット中..."
	@docker compose down -v
	@docker compose up -d
	@echo "✓ データベースをリセットしました"

# データベースのログ確認
db-logs:
	@docker compose logs -f postgres

# プロダクト管理コマンドの実行例
run-product:
	@if [ -z "$(CMD)" ]; then \
		echo "使用例:"; \
		echo "  make run-product CMD=list"; \
		echo "  make run-product CMD=show NAME=ecommerce"; \
	else \
		./bin/dev-rag product $(CMD) $(if $(NAME),--name=$(NAME),); \
	fi

# ソース管理コマンドの実行例
run-source:
	@if [ -z "$(CMD)" ]; then \
		echo "使用例:"; \
		echo "  make run-source CMD=list"; \
		echo "  make run-source CMD=list PRODUCT=ecommerce"; \
		echo "  make run-source CMD=show NAME=backend-api"; \
	else \
		./bin/dev-rag source $(CMD) \
			$(if $(NAME),--name=$(NAME),) \
			$(if $(PRODUCT),--product=$(PRODUCT),); \
	fi

# インデックスコマンドの実行例（Gitソース）
run-index:
	@if [ -z "$(URL)" ] || [ -z "$(PRODUCT)" ]; then \
		echo "使用例: make run-index URL=git@github.com:user/repo.git NAME=backend-api PRODUCT=ecommerce"; \
		echo "必須パラメータ: URL（GitリポジトリURL）、PRODUCT（プロダクト名）"; \
		echo "オプション: NAME=ソース名（省略時はURL末尾を使用） REF=ブランチ名 FORCE_INIT=true"; \
	else \
		./bin/dev-rag index git \
			--url=$(URL) \
			--product=$(PRODUCT) \
			$(if $(NAME),--name=$(NAME),) \
			$(if $(REF),--ref=$(REF),) \
			$(if $(FORCE_INIT),--force-init,); \
	fi

# Wiki生成コマンドの実行例（プロダクト単位）
run-wiki:
	@if [ -z "$(PRODUCT)" ]; then \
		echo "使用例: make run-wiki PRODUCT=ecommerce"; \
		echo "必須パラメータ: PRODUCT（プロダクト名）"; \
		echo "オプション: OUT=出力ディレクトリ CONFIG=設定ファイルパス"; \
	else \
		./bin/dev-rag wiki generate \
			--product=$(PRODUCT) \
			$(if $(OUT),--out=$(OUT),) \
			$(if $(CONFIG),--config=$(CONFIG),); \
	fi

# HTTPサーバの起動
run-server:
	@echo "HTTPサーバを起動中..."
	@./bin/dev-rag server start $(if $(PORT),--port=$(PORT),)

# 全ビルド（クリーン→ビルド）
rebuild: clean build

# リンター実行（golangci-lintが必要）
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "リンターを実行中..."; \
		golangci-lint run ./...; \
		echo "✓ リンター完了"; \
	else \
		echo "golangci-lint がインストールされていません"; \
		echo "インストール: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# フォーマット
fmt:
	@echo "コードをフォーマット中..."
	@go fmt ./...
	@echo "✓ フォーマット完了"

# sqlcでコード生成
sqlc-generate:
	@echo "SQLクエリからGoコードを生成中..."
	@go tool sqlc generate
	@echo "✓ コード生成完了: internal/module/*/adapter/pg/sqlc/"

# ビルド + テスト
all: build test
	@echo "✓ ビルドとテストが完了しました"
