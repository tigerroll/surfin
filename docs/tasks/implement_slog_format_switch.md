# タスクチケット: slog ログフォーマット切り替え機能の実装

## 概要

このタスクは、Go 1.21で導入された `log/slog` を利用したロギングにおいて、ログの出力フォーマット（JSONまたはText）を `application.yaml`
の設定で切り替えられるようにするためのものです。既存のアプリケーションの動作を維持しながら、段階的に機能を追加します。

## 進捗状況

| #   | タスク                                                               | ステータス | 完了日 |
| :-- | :------------------------------------------------------------------- | :--------- | :----- |
| 1   | `Config` 構造体にログフォーマット設定を追加                          | 完了       |        |
| 2   | `application.yaml` にログフォーマット設定を追加                      | 完了       |        |
| 3   | `pkg/batch/support/util/logger` でログフォーマットを適用             | 完了       |        |
| 4   | アプリケーション起動時にロガー設定を適用                             | 完了       |        |

## タスクリスト

### タスク 1: `Config` 構造体にログフォーマット設定を追加

*   **目的**: `application.yaml` からログフォーマット設定を読み込めるように、設定構造体に新しいフィールドを追加します。
*   **変更ファイル**:
    *   `pkg/batch/core/config/config.go`
*   **変更内容**:
    *   `pkg/batch/core/config/config.go` 内の `LoggingConfig` 構造体に、ログフォーマットを保持するフィールドを追加します。
        ```go
        type LoggingConfig struct {
            // ... 既存のフィールド
            Format string `yaml:"format"` // "json" または "text"
        }
        ```
*   **テスト方法**:
    *   アプリケーションをビルドし、エラーが発生しないことを確認します。
    *   `application.yaml` に一時的に `log_format` を追加し、デバッグログなどで `Config` 構造体が正しくパースされているか（例: `fmt.Printf("%+v\n", cfg.Surfin.System.Logging.Format)`）確認します。

### タスク 2: `application.yaml` にログフォーマット設定を追加

*   **目的**: 新しいログフォーマット設定項目を `application.yaml` に記述し、デフォルト値を設定します。
*   **変更ファイル**:
    *   `example/weather/cmd/weather/resources/application.yaml`
    *   `example/hello-world/cmd/hello-world/resources/application.yaml`
*   **変更内容**:
    *   `example/weather/cmd/weather/resources/application.yaml` および `example/hello-world/cmd/hello-world/resources/application.yaml` の `surfin.system.logging` セクションに `format: "json"` を追加します。
        ```yaml
        surfin:
          system:
            logging:
            # ... 既存の設定
              format: "json" # デフォルトはJSON形式
        ```
*   **テスト方法**:
    *   アプリケーションを起動し、エラーが発生しないことを確認します。
    *   この時点ではログ出力形式は変わりませんが、設定が正しく読み込まれることを確認します。

### タスク 3: `pkg/batch/support/util/logger` でログフォーマットを適用

*   **目的**: `logger` パッケージが、設定されたログフォーマットに基づいて `slog` のハンドラー（JSONまたはText）を切り替えるようにします。
*   **変更ファイル**:
    *   `pkg/batch/support/util/logger/logger.go`
*   **変更内容**:
    *   `pkg/batch/support/util/logger/logger.go` に、現在のログフォーマットを保持するグローバル変数 `currentLogFormat` を追加します。
    *   `SetLogFormat(format string)` 関数を新規作成し、`currentLogFormat` を更新し、`slogLogger` を再構築するヘルパー関数 `updateSlogLogger()` を呼び出すようにします。
    *   `SetLogLevel(level string)` 関数も `currentLogLevel` を更新し、`updateSlogLogger()` を呼び出すように修正します。
    *   `updateSlogLogger()` ヘルパー関数内で、`currentLogFormat` の値に応じて `slog.NewJSONHandler` または `slog.NewTextHandler` を選択し、`slogLogger` を初期化するロジックを実装します。
    *   `init()` 関数では、`updateSlogLogger()` を呼び出して、デフォルトの `INFO` レベルと `JSON` フォーマットで `slogLogger` を初期化するようにします。
*   **テスト方法**:
    *   `pkg/batch/support/util/logger/logger_test.go` に、`SetLogFormat` を呼び出した後にログ出力形式が切り替わることを確認するテストケースを追加します。
    *   アプリケーションを起動し、`application.yaml` の `log_format` を変更することで、ログ出力形式が切り替わることを手動で確認します。

### タスク 4: アプリケーション起動時にロガー設定を適用

*   **目的**: アプリケーションの起動時に `application.yaml` から読み込んだログレベルとログフォーマットを `logger` パッケージに適用します。
*   **変更ファイル**:
    *   `example/weather/cmd/weather/main.go`
    *   `example/hello-world/cmd/hello-world/main.go`
*   **変更内容**:
    *   `Config` オブジェクトが完全にロードされた後、`logger.SetLogLevel(cfg.Surfin.System.Logging.Level)` と `logger.SetLogFormat(cfg.Surfin.System.Logging.Format)` を呼び出すようにします。
*   **テスト方法**:
    *   `application.yaml` の `log_format` を `"json"` と `"text"` に切り替えてアプリケーションを起動し、それぞれの形式でログが出力されることを確認します。
    *   `log_level` も変更し、ログレベルフィルタリングが正しく機能することも確認します。
