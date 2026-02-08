# slog へのロギング基盤移行戦略

## 1. 目的

本ドキュメントは、現在のロギング基盤をGo 1.21で導入された標準ライブラリ `log/slog` へ移行する際の設計と戦略を記述します。主な目的は以下の通りです。

*   **構造化ログの導入**: Cloud Logging などの集中型ログ管理システムでのログ分析、検索、フィルタリングを容易にするため、JSON形式の構造化ログを出力します。
*   **標準ライブラリの活用**: 外部ライブラリへの依存を減らし、Go標準ライブラリの利用により、長期的なメンテナンス性と互換性を向上させます。
*   **パフォーマンスの改善**: 大量のログ出力におけるパフォーマンス効率の向上を図ります。

## 2. 現状の課題

現在のロギング実装 (`pkg/batch/support/util/logger`) は、Go標準の `log` パッケージをラップした非構造化ログを出力しています。これにより、以下のような課題があります。

*   ログメッセージが文字列ベースであるため、プログラムによる解析や特定のフィールドでの検索が困難です。
*   Cloud Logging などのログ分析ツールで、ログの属性に基づいた高度なフィルタリングや集計を行うことができません。

## 3. 移行戦略

既存のアプリケーションコードへの影響を最小限に抑えつつ、`slog` の利点を最大限に活用するため、以下の戦略を採用します。

### 3.1. `pkg/batch/support/util/logger` の内部実装の置き換え

*   アプリケーション全体で利用されている `pkg/batch/support/util/logger` パッケージの公開API（`Debugf`, `Infof`, `Warnf`, `Errorf`, `Fatalf`）は変更しません。
*   これらの関数の内部実装を、`log/slog` パッケージの機能に置き換えます。これにより、ロガーの呼び出し元コードは一切変更する必要がありません。

### 3.2. 構造化ログの出力とフォーマット切り替え

*   `slog.NewJSONHandler` または `slog.NewTextHandler` を使用し、標準出力にJSON形式またはText形式の構造化ログを出力します。
*   `application.yaml` の設定 (`surfin.system.logging.format`) に基づいて、JSON形式とText形式を動的に切り替えられるようにします。これにより、Cloud Logging がログを自動的にパースし、構造化されたデータとして取り込むことが可能になります。

### 3.3. ログレベルのマッピング

*   既存の `LogLevel` (LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal, LevelSilent) を `slog.Level` にマッピングします。
    *   `LevelDebug` -> `slog.LevelDebug`
    *   `LevelInfo` -> `slog.LevelInfo`
    *   `LevelWarn` -> `slog.LevelWarn`
    *   `LevelError` -> `slog.LevelError`
    *   `LevelFatal` -> `slog.LevelError` (slog に `FATAL` レベルは存在しないため、`ERROR` にマッピングし、`Fatalf` 関数内で `os.Exit(1)` を呼び出してプログラムを終了させます。)
    *   `LevelSilent` -> 全てのログ出力を抑制します。

### 3.4. 早期ロギング設定の適用

*   アプリケーションの起動時に `application.yaml` からログ設定を読み込み、Fxアプリケーションの初期化前に `logger.SetLogFormat` と `logger.SetLogLevel` を呼び出すロジックを追加します。これにより、Fxの起動プロセス中に発生するログも、ユーザーが設定したフォーマットとレベルで出力されるようになります。

## 4. 変更点概要

主に `pkg/batch/support/util/logger/logger.go`、`example/weather/cmd/weather/main.go`、`example/hello-world/cmd/hello-world/main.go`、および `application.yaml` ファイルが変更の対象となります。

*   `pkg/batch/core/config/config.go`: `LoggingConfig` 構造体に `Format` フィールドを追加し、`NewConfig` でデフォルト値を設定します。
*   `pkg/batch/support/util/logger/logger.go`:
    *   `LogLevelSilent` を追加し、`updateSlogLogger` でそのハンドリングロジックを実装します。
    *   `currentLogFormat` グローバル変数を追加し、`updateSlogLogger` 関数内で `currentLogFormat` の値に応じて `slog.NewJSONHandler` または `slog.NewTextHandler` を選択するように変更します。
    *   `SetLogFormat` 関数を新規作成し、`SetLogLevel` 関数も `updateSlogLogger()` を呼び出すように修正します。
    *   `init()` 関数で `updateSlogLogger()` を呼び出すように修正します。
    *   `Fatalf` 関数で出力されるログに `slog.String("level", "FATAL")` 属性を明示的に追加します。
    *   GoDocコメントを最適化し、英語化します。
*   `application.yaml` ファイル群 (`example/weather/cmd/weather/resources/application.yaml`, `example/hello-world/cmd/hello-world/resources/application.yaml`): `surfin.system.logging` セクションに `format: "json"` を追加します。
*   `example/weather/cmd/weather/main.go` および `example/hello-world/cmd/hello-world/main.go`: Fxアプリケーションの初期化前に `application.yaml` からログ設定を読み込み、`logger.SetLogFormat` と `logger.SetLogLevel` を呼び出すロジックを追加します。また、GoDocコメントを最適化し、英語化します。

## 5. 期待される効果

*   **Cloud Logging との連携強化**: ログがJSON形式で出力されるため、Cloud Logging が自動的に構造化されたログとして取り込み、ログエクスプローラでの検索やフィルタリングが容易になります。
*   **ログ分析の容易化**: ログのキーと値のペアに基づいた高度な分析が可能になり、問題の特定や傾向の把握が迅速に行えます。
*   **メンテナンス性の向上**: Go標準ライブラリの利用により、外部依存の管理が不要になり、将来的なGoのバージョンアップにも対応しやすくなります。
*   **開発・運用効率の向上**: 開発環境では可読性の高いText形式、本番環境では機械処理に適したJSON形式と、用途に応じたログフォーマットを容易に切り替えられるようになります。

## 6. 考慮事項

*   **既存ログのフォーマット変更**: ログの出力形式が非構造化からJSON形式の構造化ログに変わります。既存のログ解析スクリプトやツールがある場合は、その変更が必要になる可能性があります。
*   **属性の活用**: 今回の移行では、既存の `Printf` スタイルのログメッセージを `slog` のメッセージフィールドにマッピングします。より詳細な構造化ログのためには、今後 `slog.Attr` を活用してキーと値のペアで情報を追加する拡張を検討できます。
