# Parquet エクスポート設計

## 1. 概要
`GenericParquetExportTasklet` は、特定の Go 構造体型に依存せず、JSL (Job Specification Language) を通じて動的に設定可能な汎用的な Parquet ファイル出力タスクレットです。

## 2. パーティショニング設計
大規模データセットを効率的にクエリするための、階層的なパーティショニング構造を定義する。
- **Hive スタイル**: `key=value` 形式のディレクトリ構造を採用する。
- **動的生成**: `GenericParquetExportTasklet` の設定により、任意のフィールドをパーティションキーとして指定可能にする。
- **階層構造**: 複数のパーティション定義をリストで受け取り、順序に従ってパスを生成する。

## 3. Parquet Writer 設計
*   **ストリーミング書き込み**: `parquet-go` ライブラリを活用し、メモリ効率を考慮した書き込みを行う。
*   **スキーマ推論**: Go の構造体から Parquet スキーマを動的に生成する。
*   **フラッシュ制御**: `ItemFlusher` インターフェースを実装し、メモリ使用量を制御する。
*   **設計意図**: `ParquetWriter` は、`parquet-go` の内部バッファリングを最大限に活用しつつ、OOM を防ぐためにチャンク単位での明示的なフラッシュをサポートします。これにより、大規模データ処理においてもメモリ使用量を一定に保つことが可能です。

### 3.1. ParquetWriter の構造と設計パターン
`ParquetWriter[T any]` は、ストレージ接続を抽象化し、パーティションキーに基づいてデータをバッファリングします。
- **設定**: `ParquetWriterConfig` により、ストレージ参照、出力ディレクトリ、圧縮タイプを制御します。
- **インターフェース**: `port.ItemWriter` を実装し、`Open`, `Write`, `Close` を通じてライフサイクルを管理します。
- **バッファリング**: `bufferedItems` マップを使用してパーティションごとにデータを蓄積し、`Close` 時に一括してストレージへアップロードします。

## 4. Generic Parquet Export Tasklet 設計
*   **目的**: データベースからデータを読み込み、Parquet 形式でストレージへエクスポートする汎用タスクレット。
*   **設計意図**:
    *   **動的設定**: JSL を通じて、DB接続、ストレージ接続、テーブル名、パーティション設定を注入する。
    *   **型安全性**: ジェネリクスを活用し、任意の構造体型に対応する。
    *   **並行処理**: Reader と Writer を分離し、パイプライン処理を行う。
    *   **動的エンティティ選択**: Fx による型特化ロジックの注入と、JSL による動的 SQL 構築により、汎用性と型安全性を両立します。

### 4.1. 実装のポイント
- **設定構造体**: `GenericParquetExportTaskletConfig` を定義し、JSL プロパティをデコードします。
- **動的 SQL 構築**: `Execute` メソッド内で、設定されたテーブル名やカラム情報に基づき SQL を動的に構築します。
- **パイプライン処理**: `SqlCursorReader` からの読み込みと `ParquetWriter` への書き込みを goroutine で並列化し、スループットを最大化します。

## 5. トラブルシューティング/最適化

### 5.1. OOM問題の解決と ItemFlusher の導入
Parquet Export において、`parquet-go` ライブラリの内部バッファが `Close` まで解放されないことに起因する OOM (Out Of Memory) 問題が発生しました。

*   **原因**: `parquet-go` の `Writer` は、`WriteStop` (または `Close`) が呼ばれるまで全てのデータをメモリ上に保持する仕様であるため。
*   **解決策**: `ItemFlusher` インターフェースを導入し、チャンクごとに明示的に `Flush` を呼び出すことでメモリを解放する設計としました。
    *   `ItemWriter` の実装が `Flush` 機能を提供できることを明示する `ItemFlusher` インターフェースを定義。
    *   `ParquetWriter` に `Flush` メソッドを実装。
    *   `GenericParquetExportTasklet` の `Writer Goroutine` 内で、バッチ処理ごとに明示的に `Flush` を呼び出すことで、メモリ使用量を制御。
