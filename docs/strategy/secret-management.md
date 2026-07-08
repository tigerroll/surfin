# シークレット管理戦略 (Secret Management Strategy)

## ステータス
Accepted (承認済み)

## 1. 目的
本ドキュメントは、Surfin フレームワークにおけるシークレット（DBパスワード、APIキー、TLS証明書等）の管理方針を定義する。
従来の環境変数依存から脱却し、クラウドネイティブな運用に最適化された、安全かつ拡張性の高いシークレット管理を実現することを目的とする。

## 検討した代替案 (Alternatives Considered)

1. **Vault や各クラウドベンダーの SDK を直接組み込む**
   - **理由**: メンテナンスコストが過大になること、および特定のインフラ環境への依存が強まるため却下。
2. **環境変数のまま運用する**
   - **理由**: セキュリティリスク（ログ流出、プロセスダンプ）が許容できないため却下。

## 2. 設計原則
1. **インフラへの責務委譲 (Infrastructure Delegation)**
   - フレームワーク側で Vault や各クラウドベンダー（AWS/GCP）の SDK を直接実装しない。
   - シークレットの取得は、インフラ層（Kubernetes CSI Driver, Cloud Run Secret Mount 等）が提供する「ファイルマウント」を第一の手段とする。
2. **型による安全性 (Type Safety)**
   - シークレットを保持する型として `SecretString` および `SecretData` を導入する。
   - これらは `Stringer` インターフェースを実装し、ログ出力時に自動的にマスク（`********`）されることで、平文の流出を物理的に防止する。
3. **依存性の逆転 (Dependency Inversion)**
   - Core パッケージは具体的なシークレット解決の実装（Adapter）に依存してはならない。
   - Core は `SecretResolver` インターフェースのみを定義し、Adapter 側で実装を提供する。

## 3. 実装戦略

### 3.1 URIスキームによる指定
YAML 設定ファイルにおいて、シークレット値は URI スキームを用いて指定する。

- `file://<path>`: 指定されたパスのファイルを読み込む（推奨）。
- `env://<var_name>`: 指定された環境変数を読み込む（ローカル開発用）。

```yaml
# データベースのパスワード
adapter:
  database:
    metadata:
      password: "file:///etc/secrets/db_password"
```

### 3.2 型定義 (`pkg/batch/core/secret/`)
ログ流出防止のため、以下の型を導入する。

- `SecretString`: 文字列用（パスワード、APIキー等）。
- `SecretData`: バイナリ用（証明書、秘密鍵等）。

これらは `String()` メソッドをオーバーライドし、ログ出力時に中身を隠蔽する。

### 3.3 解決ロジック (`pkg/batch/core/config/loader.go`)

`ConfigLoader` は YAML パース後に以下の処理を実行する。

1. 構造体をリフレクションで走査。
2. `file://` または `env://` で始まる文字列を検知。
3. `SecretResolver` インターフェースを呼び出し、値を解決（置換）。
4. 解決済みの値を `SecretString` / `SecretData` 型として構造体にセット。

### 3.4 実装ステップ (Implementation Steps)

実装は以下の順序で進める。

1. **インターフェースと型の定義 (`pkg/batch/core/secret/`)**
   - `SecretString` および `SecretData` 構造体を定義する。
   - `SecretProvider` (個別の取得戦略) および `SecretResolver` (解決の窓口) インターフェースを定義する。
   - Core はこれらのインターフェースのみに依存し、具体的な実装クラスには依存しない。

2. **プロバイダーの実装 (`pkg/batch/adapter/secret/`)**
   - `FileSecretProvider` および `EnvSecretProvider` を実装する。
   - これらを束ねる `CompositeSecretResolver` を実装し、`SecretResolver` インターフェースを満たすようにする。

3. **ConfigLoader の改修 (`pkg/batch/core/config/loader.go`)**
   - `ConfigLoader` が `SecretResolver` インターフェースを受け取るように変更する。
   - YAML 走査時に `SecretResolver.Resolve(uri)` を呼び出すことで、シークレットを解決する。

4. **依存関係の注入 (`fx`)**
   - `fx.Provide` を使用して、`CompositeSecretResolver` を `SecretResolver` インターフェースとして登録する。
   - `ConfigLoader` は `fx` を通じて注入された `SecretResolver` を利用する。

## 4. 拡張性
将来的に新しい取得方法（例: `vault://`）が必要になった場合、以下の手順で対応する。

1. `SecretProvider` インターフェースを実装した新しいプロバイダーを作成。
2. `CompositeSecretResolver` に新しいプロバイダーを追加。
3. Core や既存の Adapter のコード変更は不要。

## 5. 結論

本戦略により、アプリケーションコードは「シークレットがどこから来たか」を意識することなく、型安全に機密情報を扱うことができる。
また、インフラ構成を変更するだけで、コード修正なしに Vault やクラウドマネージドサービスへの移行が可能となる。

## 結果/影響 (Consequences)

### メリット
- **セキュリティの向上**: `SecretString` 型により、開発者が意図せずログにシークレットを出力する事故を型レベルで防げる。
- **責務の明確化**: `ConfigLoader` が解決を担うことで、Adapter はシークレットの存在を意識せず、純粋なビジネスロジックに集中できる。
- **拡張性**: 新しい取得方法（例: `vault://`）が必要になった場合、`SecretProvider` を実装して登録するだけで対応可能。

### デメリット（トレードオフ）
- **インフラ構成への依存**: ユーザーはインフラ側でシークレットをファイルとしてマウントする設定（Kubernetes Secret 等）を行う必要がある。
- **リフレクションの利用**: `ConfigLoader` でリフレクションを使用するため、構造体の定義に依存する。ただし、既存の `loadStructFromEnv` と同様のパターンであるため、一貫性は保たれる。

## 6. 移行と運用ルール
- **既存設定の移行**: 既存の `application.yaml` にある平文のシークレットは、順次 `file://` または `env://` スキームへ移行する。
- **新規開発のルール**: 今後追加されるすべてのシークレット項目は、本戦略に従い URI スキームで指定すること。平文でのハードコードは禁止とする。
- **セキュリティの限界**: `SecretString` / `SecretData` は「ログ出力」からの保護を目的としており、メモリダンプ等からの保護を保証するものではない。機密性の高い環境では、インフラ層でのアクセス制御（IAM/RBAC）を併用すること。
