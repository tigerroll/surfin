# Hourly Forecast データエクスポートタスクレットの実装

## 目的

- このドキュメントは、`hourly_forecast` テーブルからデータを読み込み、日付カラムを基準にHiveパーティション形式（`dt=YYYY-MM-DD`）でParquetファイルとしてローカルストレージにエクスポートするタスクレットの実装手順を説明します。このタスクレットは、
- 既存のバッチアプリケーションの機能拡張として追加されました。

## 全体像

このタスクレットは、以下の主要な機能を含んでいます。

1.  **データベースからのデータ読み込み**: `hourly_forecast` テーブルから時間ごとの天気予報データを取得します。
2.  **Parquet形式への変換**: 読み込んだデータをParquet形式に変換します。この際、タイムスタンプはミリ秒単位のINT64として扱われます。
3.  **Hiveパーティション形式での保存**: データを日付ごとにグループ化し、`dt=YYYY-MM-DD` のHiveパーティション形式でローカルストレージに保存します。
4.  **Fxアプリケーションへの登録**: タスクレットがFxの依存性注入コンテナに適切に登録され、ジョブフローから利用できるようにします。
5.  **JSL (`job.yaml`) の設定**: ジョブフローにタスクレットを組み込み、必要な設定（データベース接続名、ストレージ接続名、出力ディレクトリなど）を渡します。

## 実装詳細

### 1. タスクレットファイルの作成と実装

- `example/weather/internal/step/tasklet/hourly_forecast_export_tasklet.go` という名前でファイルを作成し、以下の内容を記述してください。
- このファイルには、タスクレットの構成、依存性、および主要な実行ロジックがすべて含まれています。

```go
// example/weather/internal/step/tasklet/hourly_forecast_export_tasklet.go
package tasklet

import (
    "bytes"
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/mitchellh/mapstructure"
    "github.com/xitongsys/parquet-go/parquet"
    "github.com/xitongsys/parquet-go/writer"

    weatherModel "github.com/tigerroll/surfin/example/weather/internal/domain/model"
    "github.com/tigerroll/surfin/pkg/batch/adapter/database"
    "github.com/tigerroll/surfin/pkg/batch/adapter/storage"
    coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
    "github.com/tigerroll/surfin/pkg/batch/core/application/port"
    "github.com/tigerroll/surfin/pkg/batch/core/config"
    "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
    "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
    "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// Verify that HourlyForecastExportTasklet implements the port.Tasklet interface.
var _ port.Tasklet = (*HourlyForecastExportTasklet)(nil)

// HourlyForecastExportTaskletConfig holds configuration for the HourlyForecastExportTasklet.
type HourlyForecastExportTaskletConfig struct {
    // DbRef is the name of the database connection to use for reading hourly forecast data.
    DbRef string `mapstructure:"dbRef"`
    // StorageRef is the name of the storage connection to use for exporting Parquet files.
    StorageRef string `mapstructure:"storageRef"`
    // OutputBaseDir is the base directory within the storage bucket for exported files.
    OutputBaseDir string `mapstructure:"outputBaseDir"`
}

// HourlyForecastExportTasklet is a Tasklet that exports hourly forecast data from a database
// to Parquet files in a Hive-partitioned directory structure using a local storage adapter.
type HourlyForecastExportTasklet struct {
    config                    *HourlyForecastExportTaskletConfig
    dbConnectionResolver      database.DBConnectionResolver
    storageConnectionResolver storage.StorageConnectionResolver
}

// Close is required to satisfy the Tasklet interface.
// This tasklet does not hold long-term resources, so it does nothing.
func (t *HourlyForecastExportTasklet) Close(ctx context.Context) error {
    logger.Debugf("HourlyForecastExportTasklet for DbRef '%s' closed.", t.config.DbRef)
    return nil
}

// GetExecutionContext is required to satisfy the Tasklet interface.
// This tasklet does not maintain persistent execution context, so it returns an empty context.
func (t *HourlyForecastExportTasklet) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
    return model.NewExecutionContext(), nil
}

// SetExecutionContext is required to satisfy the Tasklet interface.
// This tasklet does not maintain persistent execution context, so it does nothing.
func (t *HourlyForecastExportTasklet) SetExecutionContext(ctx context.Context, context model.ExecutionContext) error {
    return nil
}

// NewHourlyForecastExportTasklet creates a new instance of HourlyForecastExportTasklet.
func NewHourlyForecastExportTasklet(
    properties map[string]string,
    dbConnectionResolver database.DBConnectionResolver,
    storageConnectionResolver storage.StorageConnectionResolver,
) (port.Tasklet, error) { // Returns port.Tasklet.
    logger.Debugf("HourlyForecastExportTasklet builder received properties: %v", properties)

    cfg := &HourlyForecastExportTaskletConfig{}
    if err := mapstructure.Decode(properties, cfg); err != nil {
        return nil, fmt.Errorf("failed to decode properties into HourlyForecastExportTaskletConfig: %w", err)
    }

    return &HourlyForecastExportTasklet{
        config:                    cfg,
        dbConnectionResolver:      dbConnectionResolver,
        storageConnectionResolver: storageConnectionResolver,
    }, nil
}

// Execute performs the tasklet's logic.
func (t *HourlyForecastExportTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
    logger.Infof("HourlyForecastExportTasklet is executing. Config: %+v", t.config)

    dbConn, err := t.dbConnectionResolver.ResolveDBConnection(ctx, t.config.DbRef)
    if err != nil {
        return model.ExitStatusFailed, fmt.Errorf("failed to resolve DB connection '%s': %w", t.config.DbRef, err)
    }
    defer func() {
        if err := dbConn.Close(); err != nil {
            logger.Errorf("Failed to close DB connection '%s': %v", t.config.DbRef, err)
        }
    }()

    var forecasts []weatherModel.HourlyForecast
    // ExecuteQueryは、"query"キーを持つmap[string]interface{}を受け取ると、その値を生のSQLクエリとして実行します。
    // timeとcollected_atはParquetのTIMESTAMP_MILLIS型に合わせるため、BIGINTにキャストし、ミリ秒に変換します。
    err = dbConn.ExecuteQuery(ctx, &forecasts, map[string]interface{}{
        "query": "SELECT " +
            "CAST(EXTRACT(EPOCH FROM time) * 1000 AS BIGINT) AS time, " +
            "weather_code, " +
            "temperature_2m, " +
            "latitude, " +
            "longitude, " +
            "CAST(EXTRACT(EPOCH FROM collected_at) * 1000 AS BIGINT) AS collected_at " +
            "FROM hourly_forecast ORDER BY time ASC",
    })
    if err != nil {
        return model.ExitStatusFailed, fmt.Errorf("failed to query hourly_forecast data from '%s': %w", t.config.DbRef, err)
    }

    logger.Infof("Successfully fetched %d records from hourly_forecast table using DB connection '%s'.", len(forecasts), t.config.DbRef)

    if len(forecasts) == 0 {
        logger.Warnf("No hourly forecast records to export.")
        return model.ExitStatusCompleted, nil
    }

    // Group data by date.
    forecastsByDate := make(map[string][]weatherModel.HourlyForecast) // Key: YYYY-MM-DD.
    for _, forecast := range forecasts {
        // Convert int64 (milliseconds) to time.Time and format the date.
        recordTime := time.UnixMilli(forecast.Time)
        dateStr := recordTime.Format("2006-01-02")
        forecastsByDate[dateStr] = append(forecastsByDate[dateStr], forecast)
    }

    // Resolve storage connection once.
    storageConn, err := t.storageConnectionResolver.ResolveStorageConnection(ctx, t.config.StorageRef)
    if err != nil {
        return model.ExitStatusFailed, fmt.Errorf("failed to resolve storage connection '%s': %w", t.config.StorageRef, err)
    }
    defer func() {
        if err := storageConn.Close(); err != nil {
            logger.Errorf("Failed to close storage connection '%s': %v", t.config.StorageRef, err)
        }
    }()

    // Process each date group.
    for dateStr, dailyForecasts := range forecastsByDate {
        logger.Infof("Processing %d records for date %s.", len(dailyForecasts), dateStr)

        buf := new(bytes.Buffer)
        // writer.NewParquetWriterFromWriter を使用して、bytes.Buffer に直接書き込みます。
        pw, err := writer.NewParquetWriterFromWriter(buf, new(weatherModel.HourlyForecast), 1)
        if err != nil {
            return model.ExitStatusFailed, fmt.Errorf("failed to create parquet writer for date %s: %w", dateStr, err)
        }
        pw.CompressionType = parquet.CompressionCodec_SNAPPY

        for _, forecast := range dailyForecasts {
            if err = pw.Write(forecast); err != nil {
                return model.ExitStatusFailed, fmt.Errorf("failed to write record to parquet for date %s: %w", dateStr, err)
            }
        }

        // The ParquetWriter's WriteStop() method can panic.
        // Use defer recover() to catch panics and convert them into errors.
        var writeStopErr error
        func() {
            defer func() {
                if r := recover(); r != nil {
                    if err, ok := r.(error); ok {
                        writeStopErr = err
                    } else {
                        writeStopErr = fmt.Errorf("panic value: %v", r)
                    }
                    logger.Errorf("Caught panic during pw.WriteStop() (internal) for date %s: %v", dateStr, writeStopErr)
                }
            }()
            writeStopErr = pw.WriteStop()
        }()

        if writeStopErr != nil {
            var finalErrorMessage string
            func() {
                defer func() {
                    if r := recover(); r != nil {
                        finalErrorMessage = fmt.Sprintf("error.Error() method panicked: %v", r)
                        logger.Errorf("Caught panic when calling Error() on writeStopErr for date %s: %v", dateStr, r)
                    }
                }()
                finalErrorMessage = writeStopErr.Error()
            }()

            if strings.Contains(finalErrorMessage, "runtime error: invalid memory address or nil pointer dereference") {
                finalErrorMessage = "internal parquet writer error (possible data corruption or library issue)"
            } else if finalErrorMessage == "" {
                finalErrorMessage = "unknown error (Error() method returned empty string or panicked)"
            }
            return model.ExitStatusFailed, fmt.Errorf("failed to stop parquet writer for date %s: %s", dateStr, finalErrorMessage)
        }

        logger.Infof("Successfully converted %d records to Parquet format for date %s. Data size: %d bytes.", len(dailyForecasts), dateStr, buf.Len())

        // Determine the Hive partition date.
        hivePartitionPath := fmt.Sprintf("dt=%s", dateStr)

        // Generate a unique filename for the Parquet file.
        fileName := fmt.Sprintf("hourly_forecast_%s_%s.parquet",
            strings.ReplaceAll(dateStr, "-", ""), // Use YYYYMMDD format for the filename.
            time.Now().Format("150405"))          // HHMMSS format.
        objectPath := fmt.Sprintf("%s/%s/%s", t.config.OutputBaseDir, hivePartitionPath, fileName)

        // Upload the Parquet data.
        // bucketName はローカルアダプターでは BaseDir で処理されるため空文字列で良い
        err = storageConn.Upload(ctx, "", objectPath, buf, "application/x-parquet")
        if err != nil {
            return model.ExitStatusFailed, fmt.Errorf("failed to upload parquet file for date %s to '%s': %w", dateStr, objectPath, err)
        }

        logger.Infof("Successfully uploaded Parquet file for date %s to '%s' using storage connection '%s'.", dateStr, objectPath, t.config.StorageRef)
    }

    return model.ExitStatusCompleted, nil
}

// NewHourlyForecastExportTaskletBuilder creates a jsl.ComponentBuilder that builds HourlyForecastExportTasklet instances.
// This function directly returns a jsl.ComponentBuilder, simplifying the Fx provision.
func NewHourlyForecastExportTaskletBuilder(
    dbConnectionResolver database.DBConnectionResolver,
    storageConnectionResolver storage.StorageConnectionResolver,
) jsl.ComponentBuilder { // Returns jsl.ComponentBuilder directly.
    return func(
        cfg *config.Config, // cfg is not used but matches the jsl.ComponentBuilder signature.
        resolver port.ExpressionResolver, // resolver is not used but matches the jsl.ComponentBuilder signature.
        resourceProviders map[string]coreAdapter.ResourceProvider, // resourceProviders is not used but matches the jsl.ComponentBuilder signature.
        properties map[string]string,
    ) (interface{}, error) {
        // Pass the necessary dependencies to NewHourlyForecastExportTasklet.
        return NewHourlyForecastExportTasklet(properties, dbConnectionResolver, storageConnectionResolver)
    }
}
```

## 2. HourlyForecast モデルの定義

- `example/weather/internal/domain/model/hourly_forecast.go` という名前でファイルを作成し、以下の内容を記述してください。
- この構造体は、データベースからの読み込み（GORMタグ）とParquetへの書き込み（Parquetタグ）の両方に対応します。
- `Time` と `CollectedAt` フィールドは、データベースからの `BIGINT` キャストとParquetの `TIMESTAMP_MILLIS` に合わせて `int64` 型で定義されています。

```
// example/weather/internal/domain/model/hourly_forecast.go
package model

// HourlyForecast represents the hourly weather data for export.
// It includes parquet tags for serialization to Parquet format.
// It also includes GORM tags to allow direct mapping from database queries.
type HourlyForecast struct {
    Time          int64   `gorm:"column:time;primaryKey" parquet:"name=time,type=INT64,convertedtype=TIMESTAMP_MILLIS"`
    WeatherCode   int32   `gorm:"column:weather_code" parquet:"name=weather_code,type=INT32"`
    Temperature2M float64 `gorm:"column:temperature_2m" parquet:"name=temperature_2m,type=DOUBLE"`
    Latitude      float64 `gorm:"column:latitude;primaryKey" parquet:"name=latitude,type=DOUBLE"`
    Longitude     float64 `gorm:"column:longitude;primaryKey" parquet:"name=longitude,type=DOUBLE"`
    CollectedAt   int64   `gorm:"column:collected_at" parquet:"name=collected_at,type=INT64,convertedtype=TIMESTAMP_MILLIS"`
}

// TableName specifies the table name for HourlyForecast.
func (HourlyForecast) TableName() string {
    return "hourly_forecast"
}
```

## 3. Fxアプリケーションへの登録

- `example/weather/internal/app/module.go` を開き、新しいタスクレットのビルダーをFxアプリケーションに登録します。
- これにより、アプリケーション起動時にタスクレットが依存性注入コンテナに認識されます。

```
// example/weather/internal/app/module.go (抜粋)

import (
    // ... 既存のインポート
    "github.com/tigerroll/surfin/example/weather/internal/step/tasklet" // 追加
    "github.com/tigerroll/surfin/pkg/batch/adapter/storage"       // Imports the storage package.
    "github.com/tigerroll/surfin/pkg/batch/adapter/storage/local" // Imports the local storage adapter.
    "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"     // Added for jsl.ComponentBuilder.
    "github.com/tigerroll/surfin/pkg/batch/core/config/support" // Added for support.JobFactory.
    // ...
)

var Module = fx.Options(
    // ... 既存の fx.Options

    // Storage adapter related modules
    local.Module, // Add Fx module for Local Storage Adapter

    // Provide the aggregated map[string]storage.StorageAdapter and map[string]storage.StorageProvider
    fx.Provide(NewStorageConnections), // Provides map[string]storage.StorageAdapter and map[string]storage.StorageProvider.

    // Provide the StorageConnectionResolver using NewLocalConnectionResolver.
    // This ensures that the []storage.StorageProvider group is fully assembled
    // before NewLocalConnectionResolver is called.
    fx.Provide(fx.Annotate(
        local.NewLocalConnectionResolver,              // Use local.NewLocalConnectionResolver.
        fx.As(new(storage.StorageConnectionResolver)), // Provide as storage.StorageConnectionResolver interface.
    )),

    // ... 既存の fx.Provide や fx.Annotate

    // Add the tasklet module
    tasklet.Module,

    // Register the hourlyForecastExportTasklet builder with the JobFactory.
    fx.Invoke(func(p struct {
        fx.In
        JobFactory                         *support.JobFactory
        HourlyForecastExportTaskletBuilder jsl.ComponentBuilder `name:"hourlyForecastExportTasklet"`
    }) {
        // Call JobFactory's RegisterComponentBuilder method to register the builder.
        p.JobFactory.RegisterComponentBuilder("hourlyForecastExportTasklet", p.HourlyForecastExportTaskletBuilder)
        logger.Debugf("Registered 'hourlyForecastExportTasklet' with JobFactory.")
    }),
)
```

## 4. job.yaml の更新

- `example/weather/cmd/weather/resources/job.yaml` を開き、`weatherJob` の定義に新しいステップ `exportHourlyForecastStep` を追加します。
- これは既存の `fetchWeatherDataStep` の後に実行されるように設定され、タスクレットに設定値が渡されます。

```
# example/weather/cmd/weather/resources/job.yaml (抜粋)

id: weatherJob
name: Weather Data Processing Job
description: This job fetches weather data, processes it, and stores it.
incrementer: # Job parameters incrementer configuration.
  ref: "timestampIncrementer" # Generates unique parameters using a timestamp for each job execution.
listeners:
  - ref: loggingJobListener # Listener name registered in JobFactory
  - ref: jobCompletionSignaler

flow:
  start-element: migrateMetadataStep # The initial starting element of the job flow.
  elements:
    migrateMetadataStep: # Step to run framework migrations
      id: migrateMetadataStep
      tasklet:
        ref: migrationTasklet
        properties:
          dbRef: "metadata" # Database connection name used by JobRepository.
          migrationFSName: "frameworkMigrationsFS" # Name of the embedded file system for framework migrations.
          migrationDir: "postgres" # Directory within the migration FS containing PostgreSQL migration files.
          isFramework: "true" # Indicates this is a framework migration.
      listeners:
        - ref: loggingStepListener
      transitions:
        - on: COMPLETED
          to: migrateWorkloadStep # Transition to next step on success
        - on: FAILED
          fail: true

    migrateWorkloadStep: # Step to run application migrations for workload DB
      id: migrateWorkloadStep
      tasklet:
        ref: migrationTasklet
        properties:
          dbRef: "workload" # Workload database connection name.
          migrationFSName: "weatherAppFS" # Name of the embedded file system for application migrations.
          migrationDir: "postgres" # Directory within the migration FS containing PostgreSQL migration files.
          isFramework: "false" # Indicates this is an application migration.
      listeners:
        - ref: loggingStepListener
      transitions:
        - on: COMPLETED
          to: fetchWeatherDataStep # Transition to data processing step on success
        - on: FAILED
          fail: true

    fetchWeatherDataStep:
      id: fetchWeatherDataStep
      reader:
        ref: weatherItemReader
      processor:
        ref: weatherItemProcessor
      writer: # Write to database
        ref: weatherItemWriter
        properties: # Properties for the writer component.
          targetDBName: "workload" # Specifies the target database connection name for the writer.
      chunk:
        item-count: 50 # Number of items to process in a single chunk.
        commit-interval: 1
      listeners: # Step level listeners
        - ref: loggingStepListener
      item-read-listeners: # Enable item read listeners
        - ref: loggingItemReadListener
      item-process-listeners: # Enable item process listeners
        - ref: loggingItemProcessListener
      item-write-listeners: # Enable item write listeners
        - ref: loggingItemWriteListener
      skip-listeners: # Item skip listeners
        - ref: loggingSkipListener
      retry-item-listeners: # Item retry listeners
        - ref: loggingRetryItemListener
      chunk-listeners:
        - ref: loggingChunkListener
      execution-context-promotion: # Promote reader state to Job EC
        keys:
          - "reader_context" # Promotes the entire context saved by WeatherReader.
          - "decision.condition" # Promotes the condition set by the tasklet.
      transitions:
        - on: COMPLETED # If the step completes successfully
          to: exportHourlyForecastStep # Transition to exportHourlyForecastStep
        - on: FAILED # If the step fails
          fail: true

    exportHourlyForecastStep:
      id: exportHourlyForecastStep
      tasklet:
        ref: hourlyForecastExportTasklet # ステップ1.2で登録した名前
        properties: # Properties for the tasklet component.
          dbRef: workload # hourly_forecast テーブルがあるDB接続
          storageRef: local # ローカルストレージアダプターの接続名
          outputBaseDir: "exported_weather_data" # 出力ファイルのベースディレクトリ
      listeners:
        - ref: loggingStepListener
      transitions:
        - on: COMPLETED # If the step completes successfully.
          to: checkConditionDecision # Transition to checkConditionDecision.
        - on: FAILED
          fail: true # Terminate the entire job as failed

    checkConditionDecision: # ConditionalDecision element
      id: checkConditionDecision
      ref: conditionalDecision # Reference to Decision Builder
      properties:
        conditionKey: "decision.condition" # Key to retrieve from ExecutionContext
        expectedValue: "true"             # COMPLETED if it matches this value
        defaultStatus: "FAILED"           # Default status if no match or key not found
      transitions:
        - on: COMPLETED # If the value of 'decision.condition' in the ExecutionContext is "true".
          to: randomFailTaskletStep # To the existing randomFailTaskletStep
        - on: FAILED # If decision.condition is not "true", or key not found
          to: anotherRandomFailTaskletStep # To another path

    randomFailTaskletStep:
      id: randomFailTaskletStep
      tasklet:
        ref: randomFailTasklet # ref name
        properties:
          message: "Condition was TRUE! Executing randomFailTaskletStep."
      listeners: # Step level listeners
        - ref: loggingStepListener
      transitions:
        - on: COMPLETED
          end: true
        - on: FAILED
          fail: true

    anotherRandomFailTaskletStep:
      id: anotherRandomFailTaskletStep
      tasklet:
        ref: randomFailTasklet # ref name
        properties:
          message: "Condition was FALSE or not found! Executing anotherRandomFailTaskletStep."
      listeners:
        - ref: loggingStepListener
      transitions:
        - on: COMPLETED
          end: true
        - on: FAILED
          fail: true
  transition-rules: # Transition rules are defined within elements for clarity.
```

## 5. ローカルストレージアダプターの設定追加

`example/weather/cmd/weather/resources/application.yaml` を開き、`local` という名前のローカルストレージアダプターの設定を追加します。

```
# example/weather/cmd/weather/resources/application.yaml (抜粋)

surfin:
  adapter:
    database:
      # ... 既存のデータベース設定

    storage:
      local:
        type: "local"
        base_dir: "/tmp/surfin_batch_data"

  # ... 既存のバッチフレームワーク設定
````

## 6. 動作確認

上記の変更を適用した後、アプリケーションをビルドして実行してください。

```
task clean && task weather -- run
```


実行ログに、HourlyForecastExportTasklet is executing. というメッセージと、設定された dbRef, storageRef, outputBaseDir が表示されることを確認してください。また、Successfully fetched X records... や Successfully converted Y
records...、Successfully uploaded Parquet file... といったメッセージが表示され、データのエクスポートが正常に行われていることを確認してください。

```
{"time":"...","level":"INFO","msg":"HourlyForecastExportTasklet is executing. Config: {DbRef:workload StorageRef:local OutputBaseDir:exported_weather_data}"}
{"time":"...","level":"INFO","msg":"Successfully fetched X records from hourly_forecast table using DB connection 'workload'."}
{"time":"...","level":"INFO","msg":"Processing Y records for date ZZZZ-MM-DD."}
{"time":"...","level":"INFO","msg":"Successfully converted Y records to Parquet format for date ZZZZ-MM-DD. Data size: N bytes."}
{"time":"...","level":"INFO","msg":"Successfully uploaded Parquet file for date ZZZZ-MM-DD to '/tmp/surfin_batch_data/dt=ZZZZ-MM-DD/hourly_forecast_YYYYMMDD_HHMMSS.parquet' using storage connection 'local'."}
```
