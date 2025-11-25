# Application Layer Testing Guide

このディレクトリのテストパターンとベストプラクティスを説明します。

## テストの概要

- **テストカバレッジ**: 90.1%
- **テストファイル数**: 3
- **テストケース数**: 29

## テストファイル一覧

### 1. index_service_test.go
IndexServiceのユニットテストです。インデックス化のユースケースをテストします。

**テストケース**:
- `TestIndexService_IndexSource_Success`: 正常系のインデックス化
- `TestIndexService_IndexSource_MissingIdentifier`: 識別子が欠けている場合
- `TestIndexService_IndexSource_MissingProductName`: プロダクト名が欠けている場合
- `TestIndexService_IndexSource_IndexerError`: インデクサーがエラーを返す場合
- `TestIndexService_ReindexSource`: 再インデックス化の動作確認
- `TestIndexService_IndexSource_DifferentSourceTypes`: 異なるソースタイプでの動作確認

### 2. source_service_test.go
SourceServiceのユニットテストです。ソース管理のユースケースをテストします。

**テストケース**:
- `TestSourceService_GetSource_Success`: ソース取得の正常系
- `TestSourceService_GetSource_NilID`: ID未指定時のエラーハンドリング
- `TestSourceService_GetSource_RepositoryError`: リポジトリエラーのハンドリング
- `TestSourceService_GetSourceByName_Success`: 名前によるソース取得
- `TestSourceService_GetSourceByName_EmptyName`: 名前未指定時のエラーハンドリング
- `TestSourceService_ListSources_Success`: ソース一覧取得の正常系
- `TestSourceService_ListSources_NoFilter`: フィルタ未指定時のエラーハンドリング
- `TestSourceService_GetLatestSnapshot_Success`: 最新スナップショット取得
- `TestSourceService_GetLatestSnapshot_NilID`: ID未指定時のエラーハンドリング
- `TestSourceService_ListSnapshots_Success`: スナップショット一覧取得
- `TestSourceService_ListSnapshots_NilID`: ID未指定時のエラーハンドリング
- `TestSourceService_ListSnapshots_RepositoryError`: リポジトリエラーのハンドリング

### 3. product_service_test.go
ProductServiceのユニットテストです。製品管理のユースケースをテストします。

**テストケース**:
- `TestProductService_GetProduct_Success`: 製品取得の正常系
- `TestProductService_GetProduct_NilID`: ID未指定時のエラーハンドリング
- `TestProductService_GetProduct_RepositoryError`: リポジトリエラーのハンドリング
- `TestProductService_GetProductByName_Success`: 名前による製品取得
- `TestProductService_GetProductByName_EmptyName`: 名前未指定時のエラーハンドリング
- `TestProductService_ListProducts_Success`: 製品一覧取得の正常系
- `TestProductService_ListProducts_EmptyList`: 空の一覧の取得
- `TestProductService_ListProducts_RepositoryError`: リポジトリエラーのハンドリング
- `TestProductService_ListProductsWithStats_Success`: 統計付き製品一覧取得
- `TestProductService_ListProductsWithStats_RepositoryError`: リポジトリエラーのハンドリング

## テストパターン

### 1. 基本構造

```go
func TestServiceName_MethodName_Scenario(t *testing.T) {
    // Setup: テスト環境の準備
    ctx := context.Background()
    log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

    mockRepo := &testutil.MockRepository{
        MethodFunc: func(ctx context.Context, ...) (..., error) {
            // モックの振る舞いを定義
            return expectedResult, nil
        },
    }

    service := application.NewService(mockRepo, log)

    // Execute: テスト対象の実行
    result, err := service.Method(ctx, params)

    // Assert: 結果の検証
    require.NoError(t, err)
    assert.Equal(t, expectedResult, result)
}
```

### 2. モックの使用

テストヘルパーパッケージ (`internal/module/indexing/testing`) を使用:

- `MockIndexer`: Indexerのモック
- `MockSourceReader`: SourceReaderのモック
- `MockProductReader`: ProductReaderのモック

### 3. テストフィクスチャ

共通のテストデータ生成関数:

- `TestProduct(name, description)`: テスト用Product作成
- `TestSource(name, sourceType, productID)`: テスト用Source作成
- `TestSourceSnapshot(sourceID, version, indexed)`: テスト用SourceSnapshot作成
- `TestFile(snapshotID, path, language)`: テスト用File作成
- `TestChunk(fileID, ordinal, content)`: テスト用Chunk作成

### 4. テストすべき観点

各メソッドについて以下のシナリオをテスト:

1. **正常系**: 期待通りの結果が返されること
2. **バリデーション**: 不正な入力時に適切なエラーが返されること
3. **エラーハンドリング**: 依存コンポーネントのエラーが適切に処理されること
4. **境界値**: 空の結果、Nilの値などの境界ケース

### 5. アサーション

- `require.NoError(t, err)`: エラーがないことを確認（失敗時即座にテスト終了）
- `assert.NoError(t, err)`: エラーがないことを確認（失敗してもテスト継続）
- `assert.Equal(t, expected, actual)`: 値の一致を確認
- `assert.Nil(t, value)`: nilであることを確認
- `assert.Contains(t, str, substr)`: 文字列の包含を確認
- `assert.Len(t, collection, length)`: コレクションの長さを確認
- `assert.ErrorIs(t, err, target)`: エラーの型を確認

## テストの実行

### 全テストの実行

```bash
go test ./internal/module/indexing/application/
```

### 詳細表示

```bash
go test -v ./internal/module/indexing/application/
```

### カバレッジ確認

```bash
go test -cover ./internal/module/indexing/application/
```

### カバレッジレポート

```bash
go test -coverprofile=coverage.out ./internal/module/indexing/application/
go tool cover -html=coverage.out
```

## 他のモジュールへの適用

このテストパターンは他のモジュール（wiki、search等）にも適用できます:

1. `internal/module/xxx/testing/` ディレクトリにモックとフィクスチャを作成
2. 各サービスのテストファイルを作成
3. 正常系、異常系、境界値のテストを実装
4. 90%以上のカバレッジを目指す

## ベストプラクティス

### DO（推奨）

- テストは独立して実行可能にする
- モックを適切に使用する
- テストコードも保守性を重視
- 重要なシナリオを優先
- 明確なテスト名を使用
- Setup-Execute-Assert パターンを使用

### DON'T（非推奨）

- データベースに依存するテスト（統合テストとして分離）
- 外部APIに依存するテスト（モックを使用）
- 長時間かかるテスト
- 他のテストに依存するテスト
- テスト順序に依存するテスト

## 今後の拡張

1. **統合テスト**: 実際のDBを使用したテスト（`integration_test.go`）
2. **E2Eテスト**: CLIコマンドの実行テスト
3. **パフォーマンステスト**: 大量データでの性能測定
4. **並行処理テスト**: ゴルーチンの安全性確認
