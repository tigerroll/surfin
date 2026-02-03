# タスクチケット: データベース設定戦略の実装

## 概要

このタスクは、`docs/strategy/database_config_strategy.md` で定義されたデータベース設定の設計戦略を、既存のアプリケーションの動作を維持しながら段階的に実装するためのものです。各ステップは、可能な限り独立してテストできるように構成されています。

## 進捗状況

| #   |タスク                                                                             | ステータス | 完了日 |
| :---|:--------------------------------------------------------------------------------- | :--------- | :----- |
| 1   | `application.yaml` の設定構造変更                                                 | 未着手     |        |
| 2   | `Config` 構造体のYAMLタグ変更                                                     | 未着手     |        |
| 3   | `loader.go` でのマップキー文字列変換と `mapstructure` 導入                        | 未着手     |        |
| 4   | `gorm/provider.go` でのデータベース設定アクセスパス修正と `WeaklyTypedInput` 設定 | 未着手     |        |
| 5   | その他の箇所での `WeaklyTypedInput` 設定                                          | 未着手     |        |

## タスクリスト

### タスク 1: `application.yaml` の設定構造変更

*   **目的**: 設定ファイル (`application.yaml`) のデータベース設定を、新しい設計戦略 (`surfin.adapter.database`) に合わせて変更します。
*   **変更ファイル**:
    *   `example/weather/cmd/weather/resources/application.yaml`
*   **変更内容**:
    *   `surfin` の直下にある `database` キーを `adapter` キーの下に移動し、さらにその下に `database` キーをネストさせます。
*   **テスト方法**:
    *   YAMLファイルの構文が正しいことを確認します。この変更だけではアプリケーションは正常に起動しない可能性がありますが、次のステップで解消されます。
    *   `task clean && task weather -- run` を実行し、エラーが発生することを確認します（これは予期された動作です）。
*   **備考**: このステップは、Goのコード変更なしに設定ファイルの意図する構造を反映させるための最初のステップです。

### タスク 2: `Config` 構造体のYAMLタグ変更

*   **目的**: `pkg/batch/core/config/config.go` 内の `SurfinConfig` 構造体で、`AdapterConfigs` フィールドが `application.yaml` の `adapter` キーを正しく参照するようにYAMLタグを変更します。
*   **変更ファイル**:
    *   `pkg/batch/core/config/config.go`
*   **変更内容**:
    *   `SurfinConfig` 構造体内の `AdapterConfigs` フィールドのYAMLタグを `yaml:"database"` から `yaml:"adapter"` に変更します。
*   **テスト方法**:
    *   `task clean && task weather -- run` を実行し、アプリケーションが起動することを確認します。まだ `metadata DBConnection not found` などのエラーが出る可能性がありますが、`AdapterConfigs` に `adapter` 配下のデータが読み込まれる準備が整います。
    *   ログレベルを `DEBUG` に設定し、`pkg/batch/core/config/loader.go` の `mergeSurfinConfig` 関数内で `dest.AdapterConfigs` の内容が期待通り（`database` キーを含むマップ）になっているか確認します。

### タスク 3: `loader.go` でのマップキー文字列変換と `mapstructure` 導入

*   **目的**: YAMLのアンマーシャル時に `map[interface{}]interface{}` となるキーを `string` に変換し、`mapstructure` を使用して `Config` 構造体へ確実にデコードするように `loader.go` を修正します。これにより、ネストされたマップのキーも正しく扱えるようになります。
*   **変更ファイル**:
    *   `pkg/batch/core/config/loader.go`
*   **変更内容**:
    *   `convertMapKeysToStrings` 関数を `loader.go` に追加します。
    *   `loadConfig` 関数内で、YAMLを直接 `Config` 構造体にアンマーシャルするのではなく、一度 `map[string]interface{}` にアンマーシャルし、`convertMapKeysToStrings` で処理した後、`mapstructure.Decode` を使って `Config` 構造体にデコードするようにロジックを変更します。
*   **テスト方法**:
    *   `task clean && task weather -- run` を実行し、アプリケーションが起動することを確認します。
    *   ログレベルを `DEBUG` に設定し、`pkg/batch/core/config/loader.go` の `mergeSurfinConfig` 関数内で `dest.AdapterConfigs` の内容が、`map[string]interface{}` の `database` キーの下にさらにデータベース設定が正しくネストされていることを確認します。

### タスク 4: `gorm/provider.go` でのデータベース設定アクセスパス修正と `WeaklyTypedInput` 設定

*   **目的**: `gorm/provider.go` が `Config` 構造体からデータベース設定を正しく取得できるようにアクセスパスを修正し、`mapstructure.Decode` の柔軟性を高めます。
*   **変更ファイル**:
    *   `pkg/batch/adapter/database/gorm/provider.go`
*   **変更内容**:
    *   `BaseProvider.createAndStoreConnection` 関数内で、`p.cfg.Surfin.AdapterConfigs` マップから `database` キーを介して設定を取得するように変更します。
    *   `mapstructure.Decode` の設定に `WeaklyTypedInput: true` を追加します。
*   **テスト方法**:
    *   `task clean && task weather -- run` を実行し、アプリケーションが正常に起動し、`metadata DBConnection not found in aggregated map` や `No DBProvider found for database type ''` といったデータベース接続に関するエラーが解消されることを確認します。

### タスク 5: その他の箇所での `WeaklyTypedInput` 設定

*   **目的**: データベース設定をデコードする他の箇所でも `mapstructure.Decode` の柔軟性を高め、潜在的な型変換エラーを防ぎます。
*   **変更ファイル**:
    *   `pkg/batch/core/application/usecase/module.go`
    *   `pkg/batch/core/config/bootstrap.go` (このファイルがプロジェクトに存在する場合)
*   **変更内容**:
    *   `pkg/batch/core/application/usecase/module.go` の `provideAllDBConnections` 関数内で、`mapstructure.Decode` の設定に `WeaklyTypedInput: true` を追加します。
    *   `pkg/batch/core/config/bootstrap.go` の `runFrameworkMigrationsHook` 関数内で、`mapstructure.Decode` の設定に `WeaklyTypedInput: true` を追加します。
*   **テスト方法**:
    *   `task clean && task weather -- run` を実行し、アプリケーションが正常に起動し、すべてのデータベース関連の初期化（マイグレーション、JobOperatorの初期化など）が問題なく行われることを確認します。

---
