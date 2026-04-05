# Web Proxy Adapter 実装計画

このドキュメントは、Web Proxy Adapter の実装を段階的に進めるための計画を定義します。

## 1. WebProxyConfig およびコアインターフェース/構造体の定義

### 目的
Web Proxy Adapter の設定構造体と、基本的なインターフェースおよび構造体を確立します。

### 変更ファイル
*   `pkg/batch/adapter/webproxy/config/config.go`:
    *   `WebProxyConfig` 構造体を定義します。
*   `pkg/batch/adapter/webproxy/webproxy.go` (新規作成):
    *   `WebProxyConnection` 構造体を定義し、`adapter.ResourceConnection` インターフェースを実装します。
    *   `WebProxyProvider` 構造体を定義し、`adapter.ResourceProvider` インターフェースを実装します。
    *   `WebProxyConnection` の `http.RoundTripper` インターフェースの骨格を定義します。

## 2. JobFactory との連携

### 目的
`JobFactory` が `WebProxyProvider` インスタンスを認識し、提供できるようにします。

### 変更ファイル
*   `pkg/batch/adapter/webproxy/module.go` (新規作成):
    *   `WebProxyProvider` を Fx の依存性注入グラフに提供するための Fx モジュールを定義します。
*   `pkg/batch/core/config/support/jobfactory.go`:
    *   `NewJobFactory` 関数が Fx のグループ機能を通じて `WebProxyProvider` インスタンスを収集し、`resourceProviders` マップに格納できるようにします。
*   `main.go` (またはアプリケーションのエントリポイント):
    *   新規作成した `webproxy` モジュールを Fx アプリケーションに追加します。

## 3. WebProxyConnection の基本機能実装 (HTTP Client & RoundTripper)

### 目的
`WebProxyConnection` が `http.RoundTripper` として機能し、リクエストを透過的に通過させる基本的なメカニズムを実装します。これは、すべての認証タイプの基盤となります。

### 変更ファイル
*   `pkg/batch/adapter/webproxy/webproxy.go`:
    *   `WebProxyConnection` に `RoundTripper` インターフェースの具体的な実装を追加し、内部の `http.Client` を通じてリクエストを転送するようにします。

## 4. API Key 認証の実装

### 目的
`WebProxyConnection` 内で API Key 認証をサポートします。

### 変更ファイル
*   `pkg/batch/adapter/webproxy/webproxy.go`:
    *   `RoundTripper` のロジックを修正し、`WebProxyConfig` に基づいて API Key を HTTP ヘッダー、クエリパラメータ、または認証ヘッダーに適用する処理を追加します。

## 5. OAuth2 Bearer 認証の実装

### 目的
`WebProxyConnection` 内で OAuth2 Client Credentials フロー（設計書に記載の通り）をサポートします。

### 変更ファイル
*   `pkg/batch/adapter/webproxy/webproxy.go`:
    *   トークン取得とリフレッシュのロジックを実装し、取得した Bearer トークンを `Authorization` ヘッダーに適用する処理を追加します。

## 6. HMAC/Signature 認証の実装

### 目的
`WebProxyConnection` 内で HMAC/Signature 認証をサポートします。

### 変更ファイル
*   `pkg/batch/adapter/webproxy/webproxy.go`:
    *   署名生成ロジックを実装し、`ItemReader` から `context` 経由で渡される署名対象文字列などの情報に基づいて署名を計算し、HTTP ヘッダーに適用する処理を追加します。

## 7. テストとドキュメントの更新

### 目的
実装された機能が正しく動作することを確認し、必要に応じて関連ドキュメントを更新します。

### 変更ファイル
*   `pkg/batch/adapter/webproxy/webproxy_test.go` (新規作成):
    *   `WebProxyConnection` および `WebProxyProvider` の単体テストを追加します。
*   `pkg/batch/adapter/webproxy/config/config_test.go` (新規作成):
    *   `WebProxyConfig` の設定読み込みに関する単体テストを追加します。
*   `example/` ディレクトリ内の既存のジョブ定義または新規ジョブ定義 (例: `example/weather/jobs/weather_api_job.jsl`):
    *   Web Proxy Adapter を使用するジョブの JSL 定義例を追加します。
*   `docs/strategy/web_proxy_adapter_design.md`:
    *   実装中に判明した設計上の変更点や詳細があれば、このドキュメントを更新します。
