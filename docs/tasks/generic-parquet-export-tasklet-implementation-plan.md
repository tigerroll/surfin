# GenericParquetExportTasklet 実装計画

この計画は、`docs/strategy/surfin-generic-parquet-export-tasklet-design.md` に記載されている `GenericParquetExportTasklet` の内容を段階的に実装するためのものです。機能ごとに論理的なステップで実装を進めます。

1.  **`GenericParquetExportTaskletConfig` 構造体の定義**
    *   `GenericParquetExportTasklet` の設定を保持する構造体 `GenericParquetExportTaskletConfig` を定義します。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go` (新規作成または既存ファイルに追加)

2.  **`GenericParquetExportTasklet[T]` 構造体の定義**
    *   `port.Tasklet` インターフェースを実装する主要な構造体 `GenericParquetExportTasklet[T]` を定義します。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go`

3.  **`NewGenericParquetExportTasklet` 関数の初期実装**
    *   `GenericParquetExportTasklet` の新しいインスタンスを作成する `NewGenericParquetExportTasklet` 関数のシグネチャと、`properties` のデコード、必須設定の基本的なバリデーションロジックを実装します。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go`

4.  **`NewGenericParquetExportTaskletBuilder` 関数の実装**
    *   JSL の `ComponentBuilder` シグネチャに適合し、Fx (Dependency Injection) コンテナを通じてタスクレットを登録するための `NewGenericParquetExportTaskletBuilder` 関数を実装します。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go`

5.  **`port.Tasklet` インターフェースメソッドのスケルトン実装**
    *   `Open`, `Execute`, `Close`, `SetExecutionContext`, `GetExecutionContext` メソッドのシグネチャを定義し、初期のダミー実装（例: エラーを返さない、空の処理）を行います。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go`

6.  **`NewGenericParquetExportTasklet` における `ParquetWriter` の初期化**
    *   `NewGenericParquetExportTasklet` 関数内で、`writerComponent.NewParquetWriter` を呼び出し、`parquetWriter` フィールドを初期化するロジックを実装します。`itemPrototype` と `parquetCompressionType` の利用を含めます。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go`

7.  **`NewGenericParquetExportTasklet` における `partitionKeyFunc` の動的生成**
    *   `NewGenericParquetExportTasklet` 関数内で、JSL の `PartitionKeyColumn` と `PartitionKeyFormat` に基づいて、リフレクションを用いて動的に `partitionKeyFunc` を生成するロジックを実装します。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go`

8.  **`Execute` メソッドにおけるデータベース接続の解決と SQL クエリの構築**
    *   `Execute` メソッド内で、`dbConnectionResolver` を使用してデータベース接続を解決し、`GenericParquetExportTaskletConfig` の設定（`TableName`, `SQLSelectColumns`, `SQLOrderBy`）に基づいて動的に SQL
クエリを構築するロジックを実装します。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go`

9.  **`Execute` メソッドにおける `SqlCursorReader` の初期化**
    *   `Execute` メソッド内で、`readerComponent.NewSqlCursorReader` を初期化し、Fx から注入された `scanFunc` を渡すロジックを実装します。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go`

10. **`Execute` メソッドにおける並行処理パイプラインの実装**
    *   `Execute` メソッド内で、goroutine とチャネルを用いて、`SqlCursorReader` からの読み込みと `ParquetWriter` への書き込みを行う並行処理パイプラインを実装します。両方の goroutine の完了を待つロジックを含めます。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go`

11. **エラーハンドリングとリソース解放の最終確認**
    *   実装全体を通して、設計ドキュメントに記載されているエラーハンドリング（特にリフレクションや I/O 関連）が適切に行われているかを確認し、必要に応じて修正します。
    *   `Close` メソッドが `ParquetWriter` のリソース解放を適切に処理しているか（または `Execute` 内で `defer` されているか）を確認します。
    *   関連ファイル: `pkg/batch/component/step/tasklet/parquet_export_tasklet.go`
