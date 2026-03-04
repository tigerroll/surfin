### Parquetファイル名の柔軟なフォーマット指定に関する実装計画

**1. 目的**

Parquetファイル出力において、ファイル名のフォーマットをJSL（Job Specification Language）を通じてユーザーが柔軟に指定できるようにする。これにより、ファイル名に任意の文字列、テーブル名、タイムスタンプ、UUID、ランダム文字列などを組み合わせ、その順序も制御可能とする。

**2. 対象ファイル**

*   `pkg/batch/component/step/writer/parquet_writer.go`
*   `pkg/batch/component/tasklet/generic/parquet_export_tasklet.go`

**3. 変更内容**

**3.1. `pkg/batch/component/step/writer/parquet_writer.go` の変更**

*   **`ParquetWriterConfig` 構造体の拡張**:
    *   `FileNameFormat string` フィールドの説明を更新。これはファイル名のフォーマットパターンを定義する文字列（例: `"#{tableName}_#{timestamp}.parquet"`）となる。
    *   `TableName string` フィールドを追加。これはファイル名に含めるテーブル名を指定する。

    ```go
    // pkg/batch/component/step/writer/parquet_writer.go
    type ParquetWriterConfig struct {
    	// ... 既存のフィールド ...
    	// FileNameFormat is the format pattern for the output Parquet file name (e.g., "data_#{tableName}_#{timestamp}_#{random}.parquet").
    	// Supported placeholders: #{tableName}, #{timestamp}, #{uuid}, #{random}, #{sequence}.
    	FileNameFormat string `mapstructure:"fileNameFormat"` // ファイル名のフォーマットパターン (例: "data_#{tableName}_#{timestamp}_#{random}_#{sequence}.parquet")
    	TableName      string `mapstructure:"tableName"`      // ファイル名に含めるテーブル名
    }
    ```

*   **`NewParquetWriter` 関数の変更**:
    *   `properties` マップから `FileNameFormat` および `TableName` の設定をデコードし、`ParquetWriterConfig` に格納するように変更する。

    ```go
    // pkg/batch/component/step/writer/parquet_writer.go
    func NewParquetWriter[T any](
    	name string,
    	properties map[string]interface{},
    	storageConnectionResolver storage.StorageConnectionResolver,
    	itemPrototype *T,
    	partitionKeyFunc func(T) (string, error),
    ) (port.ItemWriter[T], error) {
    	var config ParquetWriterConfig
    	if err := mapstructure.Decode(properties, &config); err != nil {
    		return nil, exception.NewBatchError(
    			"writer",
    			fmt.Sprintf("Failed to decode ParquetWriter properties for '%s': %v", name, err),
    			err,
    			false,
    			false,
    		)
    	}
    	// ... 既存のバリデーションとデフォルト値の設定 ...

    	return &ParquetWriter[T]{
    		name:                      name,
    		config:                    &config, // 更新されたconfigをセット
    		storageConnectionResolver: storageConnectionResolver,
    		itemPrototype:             itemPrototype,
    		partitionKeyFunc:          partitionKeyFunc,
    		bufferedItems:             make(map[string][]T),
    		totalRecordsBuffered:      0,
    	}, nil
    }
    ```

*   **`Close` メソッド内のファイル名生成ロジックの変更**:
    *   現在ハードコードされているファイル名生成ロジックを、`w.config.FileNameFormat` を使用したプレースホルダー置換方式に変更する。
    *   サポートするプレースホルダーは以下の通りとする:
        *   `#{tableName}`: `w.config.TableName` の値に置換。
        *   `#{timestamp}`: 現在時刻を `YYYYMMDDhhmmss` 形式（例: `20060102150405`）の文字列にフォーマットして置換。
        *   `#{uuid}`: 新しいUUID（`uuid.New().String()`）を生成して置換。
        *   `#{random}`: 既存の `generateRandomString` 関数で生成されるランダム文字列に置換。
        *   `#{sequence}`: `Close` メソッド内でパーティションごとにインクリメントされるシーケンス番号（`00001` のようにゼロ埋め5桁）に置換。
    *   `FileNameFormat` が空文字列の場合、既存のデフォルトフォーマット（`data_タイムスタンプ_ランダム文字列.parquet` に相当する形式）を適用する。

    ```go
    // pkg/batch/component/step/writer/parquet_writer.go
    func (w *ParquetWriter[T]) Close(ctx context.Context) error {
    	// ... 既存のコード ...

    	// ファイル名生成ロジックの変更
    	fileName := w.config.FileNameFormat
    	if fileName == "" {
    		// フォーマットが指定されていない場合のデフォルトのファイル名
    		fileName = "data_#{timestamp}_#{random}.parquet"
    	}

    	// プレースホルダーの置換
    	// uuidパッケージのインポートが必要: "github.com/google/uuid"
    	fileName = strings.ReplaceAll(fileName, "#{tableName}", w.config.TableName)
    	fileName = strings.ReplaceAll(fileName, "#{timestamp}", time.Now().Format("20060102150405"))
    	fileName = strings.ReplaceAll(fileName, "#{random}", generateRandomString(8))
    	fileName = strings.ReplaceAll(fileName, "#{uuid}", uuid.New().String())

    	objectName := filepath.Join(w.config.OutputBaseDir, partitionKey, fileName)

    	// ... 既存のアップロードロジック ...
    }
    ```

**3.2. `pkg/batch/component/tasklet/generic/parquet_export_tasklet.go` の変更**

*   **`GenericParquetExportTaskletConfig` 構造体の拡張**:
    *   `OutputFileNameFormat string` フィールドを追加。これはJSLから `parquet_writer` に渡すファイル名フォーマットを指定する。

    ```go
    // pkg/batch/component/tasklet/generic/parquet_export_tasklet.go
    type GenericParquetExportTaskletConfig struct {
    	// ... 既存のフィールド ...
    	// OutputFileNameFormat is the format pattern for the output Parquet file name, passed to the ParquetWriter.
    	OutputFileNameFormat string `mapstructure:"outputFileNameFormat"` // Parquetファイル名のパターン (例: "data-#{tableName}-#{timestamp}-#{sequence}.parquet")
    }
    ```

*   **`NewGenericParquetExportTasklet` 関数の変更**:
    *   `properties` マップから `OutputFileNameFormat` の設定をデコードし、`GenericParquetExportTaskletConfig` に格納する。
    *   `writer.NewParquetWriter` を呼び出す際に渡す `parquetWriterProps` マップに、`OutputFileNameFormat` の値を `fileNameFormat` キーとして含める。
    *   同様に、`TableName` の値を `tableName` キーとして含める（`GenericParquetExportTaskletConfig` の `TableName` フィールドから取得）。

    ```go
    // pkg/batch/component/tasklet/generic/parquet_export_tasklet.go
    func NewGenericParquetExportTasklet[T any](
    	properties map[string]interface{},
    	dbConnectionResolver database.DBConnectionResolver,
    	storageConnectionResolver storage.StorageConnectionResolver,
    	itemPrototype *T,
    ) (port.Tasklet, error) {
    	var config GenericParquetExportTaskletConfig
    	// ... mapstructure.Decode のロジック ...

    	// Construct properties for writer.NewParquetWriter.
    	parquetWriterProps := map[string]interface{}{
    		"storageRef":      config.StorageRef,
    		"outputBaseDir":   config.OutputBaseDir,
    		"compressionType": config.ParquetCompressionType,
    		"fileNameFormat":  config.OutputFileNameFormat, // ここを追加
    		"tableName":       config.TableName,             // ここを追加
    	}

    	pw, err := writer.NewParquetWriter[T](
    		"genericParquetExportTaskletWriter",
    		parquetWriterProps,
    		storageConnectionResolver,
    		itemPrototype,
    		partitionKeyFunc,
    	)
    	// ... エラーハンドリング ...

    	return &GenericParquetExportTasklet[T]{
    		config:                    &config,
    		dbConnectionResolver:      dbConnectionResolver,
    		storageConnectionResolver: storageConnectionResolver,
    		parquetWriter:             pw,
    		partitionKeyFunc:          partitionKeyFunc,
    	}, nil
    }
    ```

**4. JSLでの設定例**

上記変更後、JSLでは以下のようにParquetファイル名を指定できるようになる。

```yaml
steps:
  - name: exportWeatherData
    tasklet:
      name: genericParquetExportTasklet
      properties:
        dbRef: "weatherDb"
        storageRef: "dataLakeStorage"
        outputBaseDir: "weather/hourly_forecast"
        tableName: "hourly_forecast_data" # ファイル名に含めるテーブル名
        outputFileNameFormat: "hourly_forecast_#{timestamp}_#{uuid}_#{sequence}.parquet" # 新しいファイル名フォーマット
        # 例: outputFileNameFormat: "#{tableName}_part_#{sequence}_#{random}.parquet"
        # 例: outputFileNameFormat: "my_custom_data.parquet" (プレースホルダーなしも可能)
        # 例: outputFileNameFormat: "data_#{timestamp}.parquet"
        # ... その他の設定 ...
```
