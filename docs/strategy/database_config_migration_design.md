# データベース設定管理の再設計

## 1. はじめに

本設計書は、バッチフレームワークにおけるデータベース関連の設定管理方法の変更について定義します。これまで `pkg/batch/core/config/config.go` に一元的に定義されていたデータベース設定を、よりアダプターの責務に合わせた配置へと移行し、フレームワークのモジュール性と疎結合性を向上させることを目的とします。

## 2. 目的

*   `pkg/batch/core/config/config.go` からデータベース固有の設定定義を分離し、`core` パッケージの責務をスリム化する。
*   データベースアダプターが自身の設定構造を自己完結的に管理できるようにする。
*   フレームワーク全体の設定構造をより汎用的にし、将来的な多様なアダプターの追加に対応しやすくする。
*   `job_repository_db_ref` のようなインフラストラクチャ関連の設定は、Jobが参照する重要なパラメータであるため、フレームワークのコア設定として `core/config.go` に維持する。

## 3. 変更点

### 3.1. 設定構造体の移動と再定義

*   **旧**: `pkg/batch/core/config/config.go` に `PoolConfig` および `DatabaseConfig` 構造体が定義されていました。`InfrastructureConfig` 構造体も存在し、`job_repository_db_ref` を保持していました。
*   **新**: `pkg/batch/adaptor/database/config/config.go` を新規作成し、`PoolConfig` および `DatabaseConfig` 構造体を移動しました。これにより、データベース関連の設定定義がデータベースアダプターのパッケージ内に集約されます。**`InfrastructureConfig` 構造体は `pkg/batch/core/config/config.go` に維持されます。**

### 3.2. `core/config.go` の変更

*   `pkg/batch/core/config/config.go` から `PoolConfig`、`DatabaseConfig` の定義を削除しました。
*   `SurfinConfig` 内の `Datasources` フィールドを削除し、代わりに汎用的な `AdaptorConfigs map[string]map[string]interface{}` フィールドを導入しました。これにより、YAML設定ファイル上では、アダプターの種類（例: `database`, `storage` など）をキーとして、その配下に各アダプター固有の設定を柔軟に記述できるようになります。
*   **`InfrastructureConfig` 構造体は削除されず、`SurfinConfig` 内の `Infrastructure` フィールドも維持されます。`job_repository_db_ref` のデフォルト値は、引き続き `InfrastructureConfig` 内で設定されます。**
*   `job_repository_db_ref` の値を取得するためのヘルパー関数 `config.GetJobRepositoryDBRef()` は追加されません。
*   **`NewConfig()` 関数におけるデフォルト値の変更**:
    *   フレームワークのデフォルト設定として、`AdaptorConfigs` 内の `database` カテゴリに `metadata` 接続のみを定義するよう変更しました。
    *   この `metadata` 接続のデフォルトタイプは `dummy` となります。これは、設定ファイルが提供されない「DB Less」モードでの動作を想定しており、`hello-world` のようなシンプルなアプリケーションが追加設定なしで動作できるようにするためです。
    *   `workload` 接続は、デフォルト設定からは削除されました。アプリケーション固有のデータベース接続が必要な場合は、YAML設定ファイルで明示的に定義する必要があります。

### 3.3. データベースアダプターでの設定解釈

*   `pkg/batch/adaptor/database/gorm/provider.go` 内の `BaseProvider` の初期化ロジックを変更しました。
*   `*config.Config` オブジェクト全体を受け取った後、`cfg.Surfin.AdaptorConfigs["database"]` からデータベース関連の生の設定マップ (`map[string]interface{}`) を抽出し、`github.com/mitchellh/mapstructure` ライブラリを使用して、`pkg/batch/adaptor/database/config/config.go` で定義された `DatabaseConfig` 構造体へデコードするようになりました。
*   これにより、アダプターは `core` パッケージから渡される汎用的な設定マップの中から、自身に必要な具体的な設定を動的に解釈・利用します。

### 3.4. インターフェースおよび実装の型変更

*   `pkg/batch/core/adaptor/database.go` 内の `DBConnection` インターフェースの `Config()` メソッドの戻り値の型を、`config.DatabaseConfig` から `adaptor_database_config.DatabaseConfig` (つまり `pkg/batch/adaptor/database/config.DatabaseConfig`) に変更しました。
*   これに伴い、`pkg/batch/adaptor/database/gorm/adapter.go` や `pkg/batch/adaptor/database/gorm/provider.go`、および各データベースドライバの `provider.go` (PostgreSQL, MySQL, SQLite) における `DatabaseConfig` の参照が、新しいパスの型に更新されました。
*   ダミー実装である `pkg/batch/adaptor/database/dummy/dummy_implementations.go` も同様に型が更新されました。

## 4. 新しい設定の構造 (YAML例)

```yaml
surfin:
  # ... その他のフレームワーク共通設定
  infrastructure: # Infrastructure settings
    job_repository_db_ref: "metadata" # JobRepositoryが使用するDB接続名
  adaptor: # アダプター設定のトップレベルキーを"adaptor"に変更
    database: # データベースアダプターの設定
      datasources:
        metadata:
          type: postgres
          host: localhost
          port: 5432
          database: batch_metadata
          user: batch_user
          password: batch_password
          sslmode: disable
          pool:
            max_open_conns: 10
            max_idle_conns: 5
            conn_max_lifetime_minutes: 5
        workload:
          type: postgres
          host: localhost
          port: 5433
          database: app_data
          user: app_user
          password: app_password
          sslmode: disable
          pool:
            max_open_conns: 10
            max_idle_conns: 5
            conn_max_lifetime_minutes: 5
    # ...
```

## 5. 利点

*   **疎結合の強化**: `core` パッケージが特定のアダプター（データベース）の詳細な設定構造に依存しなくなりました。`core` は汎用的な `map[string]interface{}` を扱うのみとなり、アダプターの実装がより独立しました。
*   **責務の明確化**: データベース固有の設定定義と解釈の責務が、`pkg/batch/adaptor/database` パッケージに明確に割り当てられました。**`job_repository_db_ref` はフレームワークのコア設定として `InfrastructureConfig` に維持され、その責務が明確化されました。**
*   **`core/config` のスリム化**: `pkg/batch/core/config/config.go` は、フレームワーク全体に共通する高レベルな設定と、汎用的なアダプター設定のコンテナとしての役割に特化し、見通しが良くなりました。
*   **拡張性**: 新しい種類のアダプターを追加する際に、`core/config.go` を変更することなく、`AdaptorConfigs` の下に新しいキーと設定構造を追加するだけで対応できるようになります。

## 6. 影響範囲

*   `pkg/batch/core/config/config.go`: `PoolConfig` および `DatabaseConfig` の削除、`Datasources` から `AdaptorConfigs` への変更。`InfrastructureConfig` は維持され、`job_repository_db_ref` のデフォルト値も維持されます。`GetJobRepositoryDBRef()` ヘルパー関数は追加されません。
*   `pkg/batch/adaptor/database` 配下の全ファイル: `DatabaseConfig` の型定義、設定解釈ロジック、および関連するインターフェースの実装。
*   `pkg/batch/component/tasklet/migration/tasklet.go`: データベース設定の取得ロジックが、新しい `AdaptorConfigs` 構造から取得するように変更が必要です。
*   `JobRepository` の初期化など、`job_repository_db_ref` を利用する箇所: `GlobalConfig.Surfin.Infrastructure.JobRepositoryDBRef` を直接参照するように変更が必要です（もし `GetJobRepositoryDBRef()` を使用していた場合）。
*   `github.com/mitchellh/mapstructure` ライブラリが新たに導入されました。
*   ダミーDB実装 (`pkg/batch/adaptor/database/dummy/dummy_implementations.go`) は、`Config()` メソッドの戻り値の型変更の影響を受けましたが、それ以外の直接的な影響はありません。
