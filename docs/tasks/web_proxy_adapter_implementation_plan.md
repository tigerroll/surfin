# Web Proxy Adapter 実装計画

このドキュメントは、Web Proxy Adapter の実装を段階的に進めるための計画を定義します。

### 進捗状況

| ステップ | 目的 | 進捗 |
| :--- | :--- | :--- |
| 1. WebProxyConfig およびコアインターフェース/構造体の定義 | Web Proxy Adapter の設定構造体と、基本的なインターフェースおよび構造体を確立します。 | 実装済み |
| 2. JobFactory との連携 | `JobFactory` が `WebProxyProvider` インスタンスを認識し、提供できるようにします。 | 実装済み |
| 3. WebProxyConnection の基本機能実装 (HTTP Client & RoundTripper) | `WebProxyConnection` が `http.RoundTripper` として機能し、リクエストを透過的に通過させる基本的なメカニズムを実装します。 | 実装済み |
| 4. API Key 認証の実装 | `WebProxyConnection` 内で API Key 認証をサポートします。 | 実装済み |
| 5. OAuth2 Bearer 認証の実装 | `WebProxyConnection` 内で OAuth2 Client Credentials フロー（設計書に記載の通り）をサポートします。 | 実装済み |
| 6. HMAC/Signature 認証の実装 | `WebProxyConnection` 内で HMAC/Signature 認証をサポートします。 | 実装済み |
| 7. テストとドキュメントの更新 | 実装された機能が正しく動作することを確認し、必要に応じて関連ドキュメントを更新します。**Web Proxy Adapter のモックサーバー機能の追加と検証を含みます。** | 進行中 |

---

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
実装された機能が正しく動作することを確認し、必要に応じて関連ドキュメントを更新します。**Web Proxy Adapter のモックサーバー機能の追加と検証を含みます。**

### 変更ファイル
*   `pkg/batch/adapter/webproxy/webproxy_test.go` (新規作成):
    *   `WebProxyConnection` および `WebProxyProvider` の単体テストを追加します。**モックサーバー機能のテストを含みます。**
*   `pkg/batch/adapter/webproxy/config/config_test.go` (新規作成):
    *   `WebProxyConfig` の設定読み込みに関する単体テストを追加します。
*   `pkg/batch/adapter/webproxy/config/config.go`:
    *   `WebProxyConfig` に `MockResponse` および `MockStatus` フィールドを追加し、`Type` に `MOCK_SERVER` をサポートします。
*   `pkg/batch/adapter/webproxy/webproxy.go`:
    *   `NewWebProxyConnection` で `MOCK_SERVER` タイプの場合に `httptest.Server` を起動するロジック、`Close()` で停止するロジック、および `GetMockServerURL()` メソッドを実装します。
    *   `WebProxyRoundTripper` にモックサーバーのレスポンスを返すロジックを追加します。
*   `main.go`:
    *   `WebProxyProvider` からモックサーバーの `WebProxyConnection` を取得し、その URL をジョブパラメータとしてジョブに渡すロジックを追加します。
*   `application.yaml`:
    *   `MOCK_SERVER` タイプのアダプター設定例を追加します。
*   `example/webapi_proxy/cmd/webapi_proxy/resources/job.yaml`:
    *   `api_url` をジョブパラメータから取得するように変更し、`web_proxy_ref` で認証アダプターを参照する設定例を追加します。
*   `docs/strategy/web_proxy_adapter_design.md`:
    *   実装中に判明した設計上の変更点や詳細があれば、このドキュメントを更新します。
*   `docs/strategy/web_proxy_adapter_mock_server_design.md`:
    *   モックサーバー機能の設計詳細を記述します。

### 7.1. モックサーバー機能の実装

Web Proxy Adapter のモックサーバー機能は、外部依存なしでテストや開発を可能にするために導入されます。これにより、外部 API の可用性やレートリミットに影響されることなく、バッチコンポーネントと Web Proxy Adapter の連携を検証できるようになります。

#### 目的
*   `mock_api_reader` が外部の API ではなく、ローカルで起動するモックサーバーにリクエストを送信できるようにします。
*   Web Proxy Adapter が付与した認証情報（API Key, HMAC など）をモックサーバー側で検証できる統合テストの基盤を提供します。

#### 設計の基本方針
*   **`WebProxyConfig` の拡張:** `pkg/batch/adapter/webproxy/config/config.go` の `WebProxyConfig` に、モックサーバーとしての振る舞いを定義するための新しい `type` (`MOCK_SERVER`) および関連フィールド (`MockResponse`, `MockStatus`) を追加します。
*   **`WebProxyConnection` の振る舞い:** `pkg/batch/adapter/webproxy/webproxy.go` の `WebProxyConnection` は、`WebProxyConfig` の `Type` が `"MOCK_SERVER"` の場合、`net/http/httptest.NewServer` を使用してローカルのモック HTTP サーバーを起動します。このサーバーは `WebProxyConnection` がアクティブな間は継続的にリッスンし、`Close()` で停止します。また、モックサーバーの URL を取得するための `GetMockServerURL()` メソッドが追加されます。
*   **役割分担:** 一つの `WebProxyConnection` インスタンスは、認証情報を付与するクライアントサイドのアダプターとして機能するか、モックサーバーとして機能するかのどちらか一方になります。

#### 利用方法
*   **`application.yaml`:** モックサーバーとして機能する Web Proxy と、認証情報を付与する Web Proxy の両方を定義します。
    ```yaml
    # 例: application.yaml
    surfin:
      adapter:
        openmeteo_mock_server:
          type: "MOCK_SERVER"
          mock_status: 200
          mock_response: "{\"message\": \"Mocked weather data\"}"
        openmeteo_api_key_adapter:
          type: "APIKEY"
          key: "your-api-key-value"
          placement: "header"
          key_name: "X-API-Key"
    ```
*   **`job.yaml`:** `ItemReader` の `web_proxy_ref` で認証情報を付与するアダプターを指定し、`api_url` でモックサーバーの URL をジョブパラメータ経由で指定します。
    ```yaml
    # 例: job.yaml (抜粋)
    reader:
      ref: "mockApiReader" # または実際のビジネスロジックを持つItemReader
      properties:
        api_url: "#{jobParameters['mockServerUrl']}" # モックサーバーのURLをジョブパラメータから取得
        web_proxy_ref: "openmeteo_api_key_adapter" # APIKEY認証を行うアダプターを参照
    ```
*   **`main.go`:** アプリケーション起動時に `WebProxyProvider` から `type: "MOCK_SERVER"` の `WebProxyConnection` を取得し、そのモックサーバーの URL をジョブパラメータとしてジョブに渡します。

この機能のより詳細な設計は `docs/strategy/web_proxy_adapter_mock_server_design.md` を参照してください。
