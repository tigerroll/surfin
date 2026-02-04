# Surfin Batch Framework アダプター層リファクタリング戦略

## 1. 目的

本ドキュメントは、Surfin Batch Framework のアダプター層における設計原則を再定義し、将来的な拡張性、疎結合性、および保守性を最大化するためのリファクタリング戦略を記述します。特に、Core層から特定の技術（データベース、ストレージなど）への直接的な依存を排除し、より抽象的な共通規格に基づくアダプター設計への移行を目指します。

## 2. 現状の課題

現在の設計では、`pkg/batch/core/adapter/adapter.go` にデータベース関連の具体的なインターフェース（`DBConnection`、`DBProvider` など）が定義されています。これにより、以下の課題が生じています。
 
*   **密結合**: Core層がデータベース固有のインターフェースに直接依存しているため、特定の技術（データベース）に密結合しています。
*   **拡張性の制約**: データベース以外の新しいリソースタイプ（例: オブジェクトストレージ、メッセージキュー）を追加する際、Core層のインターフェース定義に手を入れる必要が生じ、変更範囲が広がる可能性があります。
*   **依存関係逆転の原則 (DIP) への違反**: 高レベルモジュール（Core層）が低レベルモジュール（具体的なデータベース実装）の抽象に依存する形となり、DIPが十分に適用されていません。

## 3. 目標とする設計

目標とする設計では、アダプター層を以下の3つの主要な抽象レベルに分割します。

### 3.1. `pkg/batch/core/adapter/` の役割

`pkg/batch/core/adapter/` パッケージは、特定の技術に依存しない、**最も抽象的なリソース操作の共通規格**を定義します。

*   **`ResourceConnection` インターフェース**:
    *   あらゆる種類のリソース接続に共通する最小限の機能（例: `Close() error`, `Type() string`, `Name() string`）を定義します。
*   **`ResourceProvider` インターフェース**:
    *   あらゆる種類のリソースプロバイダーに共通する機能（例: `GetConnection(name string) (ResourceConnection, error)`, `CloseAll() error`, `Type() string`）を定義します。
*   **`ResourceConnectionResolver` インターフェース**:
    *   実行コンテキストに基づいて、汎用的な `ResourceConnection` を解決する機能（例: `ResolveConnection(ctx context.Context, name string) (ResourceConnection, error)`, `ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}) (string, error)`）を定義します。

### 3.2. `pkg/batch/adapter/{resource_type}/interfaces.go` の役割

`pkg/batch/adapter/` 配下の各サブパッケージ（例: `pkg/batch/adapter/database/`、`pkg/batch/adapter/storage/`）は、それぞれの **具体的なリソースタイプに特化した抽象（インターフェース）** を定義します。これらのインターフェースは、`pkg/batch/core/adapter/` で定義された汎用インターフェースを埋め込み、そのリソースタイプ固有のメソッドを追加します。

*   **`pkg/batch/adapter/database/interfaces.go`**:
    *   `DBExecutor` インターフェース: データベースの書き込み・読み取り操作（`ExecuteUpdate`, `ExecuteUpsert`, `ExecuteQuery`, `Count`, `Pluck` など）を定義します。これは `DBConnection` に埋め込まれます。
    *   `DBConnection` インターフェース: `core/adapter.ResourceConnection` を埋め込み、データベース固有の操作（例: `IsTableNotExistError`）を追加で定義します。また、`DBExecutor` を埋め込みます。
    *   `DBProvider` インターフェース: `core/adapter.ResourceProvider` を埋め込み、データベース固有の機能（例: `ForceReconnect`）を追加で定義します。
    *   `DBConnectionResolver` インターフェース: `core/adapter.ResourceConnectionResolver` を埋め込み、データベース接続の解決に特化したメソッド（例: `ResolveDBConnection`, `ResolveDBConnectionName`）を定義します。

    *補足: `docs/strategy/database_config_strategy.md` で示されているように、データベース設定は `surfin.adapter.database` の下に定義され、`pkg/batch/adapter/database/` 配下の実装がこの設定を具体的に解釈します。*

### 3.3. DIコンテナ (Fx) の役割

DIコンテナは、この設計において以下の役割を担います。

*   `pkg/batch/adapter/{resource_type}/` 配下で定義された**具体的な実装**（例: GORMベースの `DBConnection` 実装、S3クライアントベースの `StorageConnection` 実装）を、DIコンテナに登録します。
*   `pkg/batch/core/` 層のコンポーネントは、`pkg/batch/core/adapter/` で定義された**汎用インターフェース型**（例: `ResourceConnection`）を依存性として要求します。
*   DIコンテナは、登録された具体的な実装の中から、要求された汎用インターフェース型に適合するものを自動的に注入します。

## 4. リファクタリング計画

この目標設計を実現するためのリファクタリングは、以下のフェーズで進めます。

### フェーズ1: 汎用インターフェースの定義と既存コードの分離

1.  **`pkg/batch/core/adapter/interfaces.go` の作成**:
    *   `pkg/batch/core/adapter/` ディレクトリ内に `interfaces.go` を新規作成します。
    *   このファイルに、`ResourceConnection`、`ResourceProvider`、`ResourceConnectionResolver` の各汎用インターフェースを定義します。
    *   `DBProviderGroup` のようなデータベース固有の定数は、このファイルからは削除されます。
2.  **データベース固有インターフェースの移動と修正**:
    *   既存の `pkg/batch/core/adapter/adapter.go` を**削除**します。
    *   `pkg/batch/adapter/database/interfaces.go` を新規作成し、旧 `pkg/batch/core/adapter/adapter.go` にあった `DBExecutor`、`DBConnection`、`DBProvider`、`DBConnectionResolver` の定義をすべて移動します。
    *   移動した `DBConnection` インターフェースは、`core/adapter.ResourceConnection` と `DBExecutor` を埋め込むように修正します。
    *   移動した `DBProvider` インターフェースは、`core/adapter.ResourceProvider` を埋め込むように修正します。
    *   移動した `DBConnectionResolver` インターフェースは、`core/adapter.ResourceConnectionResolver` を埋め込むように修正します。
    *   `DBExecutor` は `DBConnection` に埋め込まれるため、直接汎用インターフェースを埋め込む必要はありません。
3.  **既存コードのインポートパスと型名の修正**:
    *   プロジェクト全体で `pkg/batch/core/adapter` をインポートしている箇所を特定します。
    *   Core層のコード（例: `pkg/batch/core/domain/repository`）で、データベース関連のインターフェース（`DBConnection` など）を直接使用している場合は、インポートパスを `github.com/tigerroll/surfin/pkg/batch/adapter/database` に変更し、型名を `database.DBConnection` のように修正します。
    *   DIコンテナのプロバイダー定義など、汎用インターフェースを扱うべき箇所では、`core/adapter.ResourceConnection` のような型を使用するように修正します。

### フェーズ2: DIコンテナの調整

1.  **プロバイダーの調整**:
    *   `pkg/batch/adapter/database/gorm/module.go` など、具体的なデータベース実装を提供するモジュールにおいて、提供する型が `database.DBConnection` や `database.DBProvider` となるように調整します。
    *   DIコンテナが、`database.DBConnection` を要求された際に、`core/adapter.ResourceConnection` としても解決できるように、必要に応じて `fx.As` を使用します。

### フェーズ3: 新しいリソースタイプのアダプター追加 (例: Storage)

1.  **`pkg/batch/adapter/storage/interfaces.go` の作成**:
    *   `pkg/batch/adapter/storage/` ディレクトリ内に `interfaces.go` を新規作成します。
    *   このファイルに、`StorageConnection`、`StorageProvider`、`StorageConnectionResolver` の各インターフェースを定義します。これらも `core/adapter.ResourceConnection` などの汎用インターフェースを埋め込みます。
2.  **具体的なストレージ実装の追加**:
    *   `pkg/batch/adapter/storage/s3/` など、具体的なストレージサービス（例: S3）の実装パッケージを作成します。
    *   このパッケージ内で、`StorageConnection` や `StorageProvider` の具体的な実装を提供し、DIコンテナに登録します。
3.  **Core層での利用**:
    *   Core層のコンポーネントがストレージ機能を利用する場合、`core/adapter.ResourceConnectionResolver` を介して `ResourceConnection` を取得し、必要に応じて型アサーションやアダプターパターンを用いて具体的な操作を行います。

## 5. 期待される効果

このリファクタリングにより、以下の効果が期待されます。

*   **疎結合**: Core層が具体的なリソースタイプの実装詳細から完全に分離され、汎用的な抽象にのみ依存するようになります。
*   **拡張性**: 新しいリソースタイプ（例: メッセージキュー、キャッシュ）を追加する際に、Core層のインターフェースを変更することなく、新しいアダプターパッケージを追加するだけで対応可能になります。
*   **保守性**: 各アダプターは自身の責務範囲に集中し、コードの変更が他の部分に与える影響が最小限に抑えられます。
*   **テスト容易性**: Core層のコンポーネントは、具体的な実装ではなく汎用インターフェースに依存するため、テスト時にモックやスタブを容易に差し込むことができます。
