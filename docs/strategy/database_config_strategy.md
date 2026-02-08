# データベース設定の設計戦略

## 1. 目的

本ドキュメントは、Surfin Batch Frameworkにおけるデータベース設定の管理方法と、関連するGo言語コードの設計戦略を定義します。主な目的は以下の通りです。

*   設定ファイルの構造とGo言語コードの整合性を確保する。
*   設定の読み込みと解釈における関心事の分離を明確にする。
*   フレームワークの柔軟性と拡張性を維持しつつ、データベース接続設定を効率的に管理する。

## 2. 設定ファイル (`application.yaml`) の構造

データベース設定は、`surfin.adapter.database` というネストされた構造で定義されます。これにより、`adapter` の下に複数のアダプター設定（将来的にデータベース以外のアダプターも追加可能）を配置し、その中に `database` 固有の設定をまとめることができます。

```yaml
surfin:
  adapter: # アダプター設定の抽象的なコンテナ
    database: # データベース固有の設定
      metadata: # データベース接続名
        type: "postgres"
        host: "postgres"
        # ... その他のデータベース設定
      workload: # 別のデータベース接続名
        type: "mysql"
        host: "mysql"
        # ... その他のデータベース設定
```

## 3. Go言語コードの設計方針

### 3.1. `pkg/batch/core/config/config.go` の役割

`pkg/batch/core/config/config.go` は、アプリケーション全体のトップレベル設定を定義します。ここでは、`adapter` 要素を抽象的な `map[string]interface{}` として扱います。

*   **`SurfinConfig` 構造体**:
    *   `AdapterConfigs map[string]interface{} `yaml:"adapter"` フィールドを持ちます。
    *   このフィールドは、`application.yaml` の `surfin.adapter` 配下のすべての設定を、キーが `string`、値が `interface{}` のマップとして保持します。
    *   `database` キーの具体的な解釈は、このパッケージの責務ではありません。

### 3.2. `pkg/batch/core/config/loader.go` の役割

`pkg/batch/core/config/loader.go` は、YAML設定ファイルを読み込み、Goの `Config` 構造体にデコードする責務を負います。

*   **YAML読み込み**:
    *   `yaml.Unmarshal` を使用して、展開されたYAML設定を直接 `Config` 構造体（`yamlConfig`）にデコードします。
    *   `Config` 構造体内の `AdapterConfigs` フィールドは `map[string]interface{}` として定義されており、`yaml.Unmarshal` はこの型に適切にマッピングします。
*   **設定のマージ**:
    *   `NewConfig()` で初期化されたデフォルト設定に、YAMLファイルから読み込んだ設定を `mergeConfig` 関数でマージします。これにより、YAMLで指定された値がデフォルト値を上書きします。
*   **環境変数による上書き**:
    *   `loadStructFromEnv` 関数を使用して、環境変数（例: `SURFIN_BATCH_POLLINGINTERVALSECONDS` や `SURFIN_ADAPTER_DATABASE_METADATA_HOST`）から設定値を読み込み、既存の設定を上書きします。これにより、環境変数による柔軟な設定変更が可能になります。特に、`loadMapOfStructsFromEnv` は `map[string]struct{}` 型のフィールド（例: `AdapterConfigs` 内のデータベース設定）を環境変数から動的にロードする役割を担います。

### 3.3. `pkg/batch/adapter/database/` 配下の役割

`pkg/batch/adapter/database/` 配下のパッケージ（例: `gorm/provider.go`）は、データベース固有の設定を具体的に解釈し、データベース接続を確立する責務を負います。

*   **具体的な値の解釈**:
    *   `DBProvider` の実装（例: `gorm/provider.go` の `BaseProvider.createAndStoreConnection`）は、`cfg.Surfin.AdapterConfigs` から `database` キーを介してデータベース設定マップを取得します。
    *   `dbConfigsMap, ok := adapterConfig.(map[string]interface{})` のように、`adapter` の下の `database` キーにアクセスし、その中の個別のデータベース設定（例: `metadata`）を `dbconfig.DatabaseConfig` 構造体にデコードします。
    *   このデコードには `github.com/mitchellh/mapstructure` ライブラリの `mapstructure.Decode` が使用され、YAMLタグに基づいて構造体フィールドへのマッピングを正確に行います。

### 3.4. その他の設定アクセス箇所

*   フレームワーク内の他のコンポーネント（例: マイグレーション処理、ジョブ実行管理）も、依存性注入を通じて提供される `*config.Config` オブジェクトを介してデータベース設定にアクセスします。
*   例えば、`pkg/batch/core/config/bootstrap.go` 内の `runFrameworkMigrationsHook` 関数（提供されたファイルには含まれていませんが、ドキュメントで言及されています）は、同様に `cfg.Surfin.AdapterConfigs["database"].(map[string]interface{})` を使用してデータベース設定マップを取得し、マイグレーション処理に利用すると考えられます。

## 4. 利点

この設計戦略には以下の利点があります。

*   **関心事の分離**:
    *   `core/config` パッケージは、設定の読み込みと抽象的な構造の保持に専念します。
    *   `adapter/database` パッケージは、データベース固有の設定解釈と接続確立に専念します。
*   **柔軟性**:
    *   `surfin.adapter` の下に `database` 以外の新しいアダプタータイプを容易に追加できます。
    *   各アダプターは、`map[string]interface{}` として抽象的に渡された設定を、自身の責務範囲で具体的に解釈できます。
*   **堅牢性**:
    *   `loader.go` での環境変数展開と、`mapstructure` の適切な使用により、YAMLの柔軟な入力形式や環境変数からの設定に対応しつつ、Goのコードで安全に設定にアクセスできます。
*   **一貫性**:
    *   設定ファイルとGoコード間のマッピングルールが明確になり、開発者が設定を理解しやすくなります。
