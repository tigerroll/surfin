# データベース設定管理移行計画

## 1. はじめに

本ドキュメントは、`docs/strategy/database_config_migration_design.md` に記載されたデータベース設定管理の再設計を、既存の動作を維持しつつ段階的に実施するための具体的なタスクと手順を定義します。
**注記**: `MigrationTasklet` は `JobRepository` のインスタンスを保持していますが、その内部ロジック（特に `Execute` メソッド）では直接 `JobRepository` の機能に依存していません。この既存の実装は、本移行計画によって変更されないものとします。

## 2. 目的

*   `pkg/batch/core/config/config.go` からデータベース固有の設定定義を安全に分離する。
*   データベースアダプターが自身の設定構造を自己完結的に管理できるようにする。
*   フレームワーク全体の設定構造をより汎用的にし、将来的な拡張性を確保する。
*   移行プロセス中に既存のバッチ処理の動作に影響を与えないことを最優先とする。

## 3. 前提条件

*   現在のコードベースが安定しており、全てのテストがパスしていること。
*   Go Modules が正しく設定されていること。
*   `github.com/mitchellh/mapstructure` ライブラリがまだ導入されていない場合、この計画の最初のステップで導入すること。

## 4. 移行ステップ

### 進捗管理テーブル

| ステップ | タスク                                            | ステータス | 備考 |
| :------- | :------------------------------------------------ | :--------- | :--- |
| 1        | 新しい設定パッケージの作成とライブラリの追加      | 0%         |      |
| 2        | `core/config.go` の変更 (段階的)                  | 0%         |      |
| 3        | データベースアダプターでの設定解釈の変更          | 0%         |      |
| 4        | インターフェースおよび実装の型変更                | 0%         |      |
| 5        | `core/config.go` から `Datasources` フィールドの削除とデフォルト値の最終調整 | 0%         |      |
| 6        | 影響範囲の修正と最終確認                          | 0%         |      |

---

以下のステップは、依存関係を考慮し、各ステップで動作確認を行いながら進めることを推奨します。

### ステップ 1: 新しい設定パッケージの作成とライブラリの追加

1.  **新しい設定パッケージディレクトリの作成**:
    `pkg/batch/adaptor/database/config/` ディレクトリを作成します。
2.  **`PoolConfig` および `DatabaseConfig` の移動**:
    `pkg/batch/core/config/config.go` から `PoolConfig` および `DatabaseConfig` 構造体の定義を切り取り、新しく作成した `pkg/batch/adaptor/database/config/config.go` ファイルに貼り付けます。
    *   `pkg/batch/adaptor/database/config/config.go` のパッケージ名を `config` とします。
3.  **`mapstructure` ライブラリの追加**:
    `go get github.com/mitchellh/mapstructure` を実行し、プロジェクトにライブラリを追加します。
4.  **ビルド確認**:
    `go build ./...` を実行し、移動によるビルドエラーがないことを確認します。この時点では、まだ参照元のパスが更新されていないため、多くのエラーが発生するはずです。これは次のステップで修正します。

### ステップ 2: `core/config.go` の変更 (段階的)

このステップでは、`SurfinConfig` に新しい `AdaptorConfigs` フィールドを追加し、既存の `Datasources` フィールドとの共存期間を設けることで、安全に移行を進めます。

1.  **`SurfinConfig` への `AdaptorConfigs` フィールドの追加**:
    `pkg/batch/core/config/config.go` の `SurfinConfig` 構造体に、以下のフィールドを追加します。
    ```go
    type SurfinConfig struct {
    	Datasources    map[string]DatabaseConfig `yaml:"datasources"` // 既存のフィールド (一時的に維持)
    	Batch          BatchConfig               `yaml:"batch"`
    	System         SystemConfig              `yaml:"system"`
    	Infrastructure InfrastructureConfig      `yaml:"infrastructure"`
    	Security       SecurityConfig            `yaml:"security"`
    	AdaptorConfigs map[string]map[string]interface{} `yaml:"adaptor"` // 新しいフィールド
    }
    ```
2.  **`NewConfig()` のデフォルト値の調整**:
    `pkg/batch/core/config/config.go` の `NewConfig()` 関数内で、`AdaptorConfigs` のデフォルト値を設定します。この時点では、既存の `Datasources` のデフォルト値も維持します。
    ```go
    func NewConfig() *Config { // 戻り値の型を *Config に修正
    	cfg := &Config{ // Config 構造体のインスタンスを作成
    		Surfin: SurfinConfig{
    			System: SystemConfig{
    				Timezone: "UTC",
    				Logging:  LoggingConfig{Level: "INFO"},
    			},
    			Batch: BatchConfig{
    				JobName:                "",
    				ChunkSize:              10,
    				StepExecutorRef:        "simpleStepExecutor",
    				MetricsAsyncBufferSize: 100,
    				ItemRetry: ItemRetryConfig{
    					MaxAttempts:     3,
    					InitialInterval: 1000,
    					RetryableExceptions: []string{
    						"*surfin/pkg/batch/support/util/exception.TemporaryNetworkError",
    						"net.OpError",
    						"context.DeadlineExceeded",
    						"context.Canceled",
    					},
    				},
    				ItemSkip: ItemSkipConfig{
    					SkipLimit: 0,
    					SkippableExceptions: []string{
    						"*surfin/pkg/batch/support/util/exception.DataConversionError",
    						"json.UnmarshalTypeError",
    					},
    				},
    			},
    			Datasources: map[string]DatabaseConfig{ // 既存のDatasourcesのデフォルト値
    				"metadata": { // metadataのみを保持し、Typeをdummyに設定
    					Type: "dummy",
    				},
    				// workloadはデフォルトから削除
    			},
    			Infrastructure: InfrastructureConfig{
    				JobRepositoryDBRef: "metadata",
    			},
    			Security: SecurityConfig{
    				MaskedParameterKeys: []string{"password", "api_key", "secret"},
    			},
    		},
    	}

    	// 新しい AdaptorConfigs のデフォルト設定を追加
    	cfg.Surfin.AdaptorConfigs = map[string]map[string]interface{}{
    		"database": {
    			"metadata": map[string]interface{}{ // datasourcesキーは不要、直接接続名
    				"type": "dummy", // dummyタイプの場合、他の設定は不要
    			},
    		},
    	}

    	return cfg
    }
    ```
    **注**: この時点では、`workload` 接続はまだ `Datasources` 側に残っている可能性があります。最終的には削除されますが、このステップでは `AdaptorConfigs` 側の `database` に `metadata` の `dummy` タイプのみを設定します。
3.  **ビルド確認**:
    `go build ./...` を実行し、ビルドエラーがないことを確認します。

### ステップ 3: データベースアダプターでの設定解釈とインターフェースの型変更

このステップでは、データベースアダプターが新しい `AdaptorConfigs` 構造から設定を読み込むように変更し、それに伴い `DBConnection` インターフェースの型も更新します。

1.  **`pkg/batch/adaptor/database/gorm/provider.go` の変更**:
    `BaseProvider.createAndStoreConnection` メソッドを修正し、`p.cfg.Surfin.Datasources` の代わりに `p.cfg.Surfin.AdaptorConfigs` から設定を読み込むようにします。
    *   `pkg/batch/adaptor/database/config` パッケージをインポートします。
    *   `createAndStoreConnection` メソッド内で、`dbConfig, ok := p.cfg.Surfin.Datasources[name]` の行を削除し、代わりに `p.cfg.Surfin.AdaptorConfigs["database"][name]` から `mapstructure.Decode` を使用して `adaptor_database_config.DatabaseConfig` にデコードするように変更します。
        ```go
        // pkg/batch/adaptor/database/gorm/provider.go (createAndStoreConnection メソッド内)
        import (
            adaptor_database_config "github.com/tigerroll/surfin/pkg/batch/adaptor/database/config"
            "github.com/mitchellh/mapstructure" // 追加
        )
        // ...
        var dbConfig adaptor_database_config.DatabaseConfig // 新しい型
        rawConfig, ok := p.cfg.Surfin.AdaptorConfigs["database"][name]
        if !ok {
            return nil, fmt.Errorf("database configuration '%s' not found in adaptor.database configs", name)
        }
        if err := mapstructure.Decode(rawConfig, &dbConfig); err != nil {
            return nil, fmt.Errorf("failed to decode database config for '%s': %w", name, err)
        }
        // ... (dbConfig を使用する後続のロジックはそのまま)
        ```
    *   `DialectorFactory` のシグネチャを `func(cfg adaptor_database_config.DatabaseConfig) (gorm.Dialector, error)` に変更します。
    *   これに伴い、`RegisterDialector` および `GetDialectorFactory` のシグネチャも変更します。
    *   各データベースドライバの `provider.go` (PostgreSQL, MySQL, SQLite) の `init()` 関数内の `gormadaptor.RegisterDialector` の呼び出しを、新しいシグネチャに合わせて修正します。
    *   `NewGormDBAdapter` 関数の `cfg` 引数の型を `adaptor_database_config.DatabaseConfig` に変更します。
    *   `GormDBAdapter` 構造体の `cfg` フィールドの型を `adaptor_database_config.DatabaseConfig` に変更します。
    *   `GormDBAdapter.Config()` メソッドの戻り値の型を `adaptor_database_config.DatabaseConfig` に変更します。
2.  **`DBConnection` インターフェースの `Config()` メソッドの戻り値の型変更**:
    `pkg/batch/core/adaptor/database.go` の `DBConnection` インターフェースの `Config()` メソッドの戻り値の型を、`config.DatabaseConfig` から `adaptor_database_config.DatabaseConfig` に変更します。
    ```go
    // pkg/batch/core/adaptor/database.go
    import adaptor_database_config "github.com/tigerroll/surfin/pkg/batch/adaptor/database/config" // 追加

    type DBConnection interface {
    	// ...
    	Config() adaptor_database_config.DatabaseConfig // 変更後
    	// ...
    }
    ```
3.  **`pkg/batch/adaptor/database/dummy/dummy_implementations.go` の修正**:
    `dummyDBConnection.Config()` メソッドの戻り値の型を `adaptor_database_config.DatabaseConfig` に変更します。
4.  **ビルド確認**:
    `go build ./...` を実行し、ビルドエラーがないことを確認します。
5.  **テストと動作確認**:
    データベース接続を必要とする既存のテストや、簡単なバッチジョブを実行し、新しい設定読み込みパスでデータベース接続が正しく確立され、動作することを確認します。
    *   YAML設定ファイルで、`surfin.adaptor.database` の形式でデータベース設定を記述し、それが正しく読み込まれることを確認します。

### ステップ 4: `MigrationTasklet` の修正

このステップでは、`MigrationTasklet` が新しい `AdaptorConfigs` 構造からデータベース設定を読み込むように変更します。

     `MigrationTasklet` の `Execute` メソッド内で、データベース設定 (`dbConfig`) を取得している箇所を、新しい `t.cfg.Surfin.AdaptorConfigs` 構造から取得するように修正します。
     具体的には、`dbConfig, ok := t.cfg.Surfin.Datasources[t.dbConnectionName]` の行を、`t.cfg.Surfin.AdaptorConfigs["database"][t.dbConnectionName]` から `mapstructure` を使用して `adaptor_database_config.DatabaseConfig` にデコードするように変更します。
     ```go
     // pkg/batch/component/tasklet/migration/tasklet.go (Execute メソッド内)
     import (
         adaptor_database_config "github.com/tigerroll/surfin/pkg/batch/adaptor/database/config"
         "github.com/mitchellh/mapstructure" // 追加
     )
     // ...
     var dbConfig adaptor_database_config.DatabaseConfig // 新しい型
     rawConfig, ok := t.cfg.Surfin.AdaptorConfigs["database"][t.dbConnectionName]
     if !ok {
         return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "Database configuration '%s' not found in adaptor.database configs", t.dbConnectionName)
     }
     if err := mapstructure.Decode(rawConfig, &dbConfig); err != nil {
         return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "Failed to decode database config for '%s': %w", name, err)
     }
     // ... (dbConfig を使用する後続のロジックはそのまま)
     ```
 4.  **ビルド確認**:
     `go build ./...` を実行し、ビルドエラーがないことを確認します。
 5.  **テストと動作確認**:
     既存のテストおよびバッチジョブを実行し、全ての機能が正常に動作することを確認します。特に、`dummy` タイプの `metadata` 接続が正しく機能すること、および `workload` 接続が必要な場合はYAMLで明示的に定義する必要があることを確認します。

### ステップ 5: `core/config.go` から `Datasources` フィールドの削除とデフォルト値の最終調整

このステップで、古い `Datasources` フィールドを完全に削除し、新しい設定構造に一本化します。

1.  **`SurfinConfig` から `Datasources` フィールドの削除**:
    `pkg/batch/core/config/config.go` の `SurfinConfig` 構造体から `Datasources map[string]DatabaseConfig` フィールドを削除します。
2.  **`NewConfig()` のデフォルト値の最終調整**:
    `pkg/batch/core/config/config.go` の `NewConfig()` 関数内で、`AdaptorConfigs` のデフォルト設定のみが残るように調整します。
    *   `metadata` 接続のデフォルトタイプが `dummy` であることを確認します。
    *   `workload` 接続のデフォルト設定が削除されていることを確認します。
3.  **ビルド確認**:
    `go build ./...` を実行し、ビルドエラーがないことを確認します。
4.  **テストと動作確認**:
    既存のテストおよびバッチジョブを実行し、全ての機能が正常に動作することを確認します。特に、`dummy` タイプの `metadata` 接続が正しく機能すること、および `workload` 接続が必要な場合はYAMLで明示的に定義する必要があることを確認します。

### ステップ 6: 影響範囲の修正と最終確認

1.  **`pkg/batch/component/tasklet/migration/tasklet.go` の修正**:
    もし `tasklet.go` 内でデータベース設定を直接取得している箇所があれば、新しい `GlobalConfig.Surfin.AdaptorConfigs` 構造から取得するようにロジックを修正します。
2.  **`job_repository_db_ref` の利用箇所の確認**:
    `JobRepository` の初期化など、`job_repository_db_ref` を利用する箇所が `GlobalConfig.Surfin.Infrastructure.JobRepositoryDBRef` を直接参照していることを確認します。もし、以前 `GetJobRepositoryDBRef()` のようなヘルパー関数を使用していた場合は、その参照を直接参照に切り替えます。
3.  **最終的なビルドとテスト**:
    全ての変更が完了した後、プロジェクト全体をビルドし、全ての単体テスト、結合テスト、およびエンドツーエンドテストを実行して、機能が完全に維持されていることを確認します。

## 5. 検証方法

*   **各ステップ後のビルド確認**: `go build ./...` を実行し、コンパイルエラーがないことを確認します。
*   **単体テストの実行**: `go test ./...` を実行し、既存の単体テストが全てパスすることを確認します。
*   **結合テストの実行**: データベース接続を伴う結合テストを実行し、データベース操作が正常に行われることを確認します。
*   **サンプルバッチジョブの実行**: `hello-world` のようなシンプルなバッチジョブや、データベースを利用する既存のバッチジョブを実際に実行し、期待通りに動作することを確認します。特に、`metadata` データベースへの接続が正しく行われることを確認します。
*   **YAML設定ファイルのテスト**: 新しいYAML設定構造（`surfin.adaptor.database`）で設定を記述し、それが正しく読み込まれ、アプリケーションに適用されることを確認します。

## 6. ロールバック手順

問題が発生した場合、以下の手順で変更を元に戻すことができます。

1.  **Git リバート**:
    `git log` で変更履歴を確認し、問題が発生した変更セットを特定します。
    `git revert <commit-hash>` または `git reset --hard <commit-hash>` を使用して、問題のない状態に戻します。
    **注**: `git reset --hard` は未コミットの変更も破棄するため、注意して使用してください。
2.  **手動でのファイル復元**:
    Git を使用しない場合、変更を加えたファイルをバックアップから復元するか、手動で以前の状態に戻します。
