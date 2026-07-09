# シークレット管理戦略 (Secret Management Strategy)

## 1. 目的
本ドキュメントは、Surfin フレームワークにおけるシークレット（DBパスワード、APIキー、TLS証明書等）の管理方針を定義する。
従来の環境変数依存から脱却し、クラウドネイティブな運用に最適化された、安全かつ拡張性の高いシークレット管理を実現することを目的とする。

## 2. 設計原則
1. **インフラへの責務委譲 (Infrastructure Delegation)**
   - フレームワーク側で Vault や各クラウドベンダーの SDK を直接実装しない。
   - シークレットの取得は、インフラ層（Kubernetes CSI Driver, Cloud Run Secret Mount 等）が提供する「ファイルマウント」を第一の手段とする。
2. **型による安全性 (Type Safety)**
   - シークレットを保持する型として `SecretString` および `SecretData` を導入する。
   - これらは `Stringer` インターフェースを実装し、ログ出力時に自動的にマスク（`********`）されることで、平文の流出を物理的に防止する。
3. **依存性の逆転 (Dependency Inversion)**
   - Core パッケージは具体的なシークレット解決の実装に依存してはならない。
   - Core は `SecretResolver` インターフェースのみを定義し、標準的なプロバイダー（File, Env）は `pkg/batch/core/secret/` に配置する。これにより、Adapter 間での循環依存や不要な依存関係を排除する。

## 3. 実装戦略

### 3.1 URIスキームによる指定
YAML 設定ファイルにおいて、シークレット値は URI スキームを用いて指定する。

- `file://<path>`: 指定されたパスのファイルを読み込む（推奨）。
- `env://<var_name>`: 指定された環境変数を読み込む（ローカル開発用）。

### 3.2 型定義とプロバイダー (`pkg/batch/core/secret/`)
ログ流出防止のための型定義および、標準的な解決ロジックを配置する。

- `SecretString`: 文字列用（パスワード、APIキー等）。
- `SecretData`: バイナリ用（証明書、秘密鍵等）。
- `FileSecretProvider`, `EnvSecretProvider`: 標準的な解決プロバイダー。

### 3.3 解決ロジック (`pkg/batch/core/config/loader.go`)
`ConfigLoader` は YAML パース後に以下の処理を実行する。

1. 構造体をリフレクションで走査。
2. `file://` または `env://` で始まる文字列を検知。
3. `SecretResolver` インターフェースを呼び出し、値を解決（置換）。
4. 解決済みの値を `SecretString` / `SecretData` 型として構造体にセット。

### 3.4 実装ステップ (Implementation Steps)

1. **インターフェースと型の定義 (`pkg/batch/core/secret/`)**
   - `SecretString`, `SecretData` 構造体を定義。
   - `SecretProvider`, `SecretResolver` インターフェースを定義。
   - 標準的なプロバイダー（`FileSecretProvider`, `EnvSecretProvider`）を実装。

2. **ConfigLoader の改修 (`pkg/batch/core/config/loader.go`)**
   - `ConfigLoader` が `SecretResolver` を利用してシークレットを解決する。

3. **依存関係の注入 (`fx`)**
   - `fx.Provide` を使用して、`SecretResolver` をアプリケーション全体に提供する。

## 4. 拡張性
将来的に新しい取得方法（例: `vault://`）が必要になった場合、`pkg/batch/infrastructure/secret/` 等に新しいプロバイダーを追加し、`fx` モジュールで登録することで、既存の Core コードを修正することなく対応可能である。
