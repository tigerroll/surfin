# Parquet ファイル名設計

## 1. 目的
出力ファイル名の柔軟な命名規則の提供。

## 2. 設計
*   **プレースホルダー**: `#{tableName}`, `#{timestamp}`, `#{uuid}`, `#{random}`, `#{sequence}` をサポート。
*   **JSL 設定**: `outputFileNameFormat` プロパティでパターンを指定可能にする。
*   **動的生成**: `Close` 時にプレースホルダーを置換してファイル名を決定する。
