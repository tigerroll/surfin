# Web Proxy Adapter モックサーバー機能設計書

### 1. 目的

本ドキュメントは、`surfin` の Web Proxy Adapter に、外部依存なしでテストや開発を可能にするローカルモックサーバー機能を追加する設計について記述します。これにより、外部 API の可用性やレートリミットに影響されることなく、バッチコンポーネントと Web Proxy Adapter の連携を検証できるようになります。

### 2. 背景

Web Proxy Adapter の実装検証および利用例として `mock_api_reader.go` を導入しましたが、これはデフォルトで外部のダミー API (`jsonplaceholder.typicode.com`) にリクエストを送信していました。テストの安定性向上と外部依存の排除のため、この `mock_api_reader` がローカルで起動するモックサーバーにリクエストを送信する仕組みが必要となりました。

また、このモックサーバー機能は、単に固定レスポンスを返すだけでなく、Web Proxy Adapter が付与した認証情報（API Key, HMAC など）をモックサーバー側で検証できるような統合テストの基盤としても機能します。

### 3. 設計の基本方針

Web Proxy Adapter のモックサーバー機能は、既存の `ResourceProvider` および `ResourceConnection` のパターンに統合されます。

#### 3.1. `WebProxyConfig` の拡張

`pkg/batch/adapter/webproxy/config/config.go` の `WebProxyConfig` に、モックサーバーとしての振る舞いを定義するための新しい `type` (`MOCK_SERVER`) および関連フィールドを追加します。

```go
// pkg/batch/adapter/webproxy/config/config.go (抜粋)
type WebProxyConfig struct {
	Type         string `yaml:"type"`             // 認証タイプ (例: "HMAC", "OAUTH2", "APIKEY", "MOCK_SERVER")
	// ... 既存の認証関連フィールド ...
	MockResponse string `yaml:"mock_response,omitempty"` // MOCK_SERVER 用の固定レスポンスボディ
	MockStatus   int    `yaml:"mock_status,omitempty"`   // MOCK_SERVER 用の固定HTTPステータスコード
	// 将来的に OpenAPI/Swagger のパスや設定など、より高度なモック機能のためのフィールドを追加可能
}
```

#### 3.2. `WebProxyConnection` の振る舞い

`pkg/batch/adapter/webproxy/webproxy.go` の `WebProxyConnection` は、`WebProxyConfig` の `Type` に応じて異なる振る舞いをします。

*   **`Type: "MOCK_SERVER"` の場合:**
    *   `NewWebProxyConnection` の中で `net/http/httptest.NewServer` を使用してローカルのモック HTTP サーバーを起動します。
    *   このモックサーバーは、`WebProxyConnection` がアクティブな間は継続的にリッスンします。
    *   `WebProxyConnection` が返す `http.Client` は、この起動したモックサーバーの URL を指すように設定されます。
    *   `WebProxyConnection.Close()` が呼ばれた際に、起動したモックサーバーを停止します (`httptest.Server.Close()`)。
    *   モックサーバーの URL を外部から取得するための `GetMockServerURL()` メソッドを `WebProxyConnection` に追加します。

*   **`Type: "APIKEY"`, `"OAUTH2"`, `"HMAC"` の場合:**
    *   既存の設計通り、`WebProxyRoundTripper` を使用して送信される HTTP リクエストに認証情報を付与するクライアントサイドのアダプターとして機能します。

#### 3.3. `WebProxyConnection` の役割分担

一つの `WebProxyConnection` インスタンスは、設定された `Type` に応じて、**認証情報を付与するクライアントサイドのアダプター**として機能するか、**モックサーバーとして機能する**かのどちらか一方になります。両方の機能を同時に果たすことはありません。

### 4. 利用方法

#### 4.1. `application.yaml` の設定例

`application.yaml` に、モックサーバーとして機能する Web Proxy と、認証情報を付与する Web Proxy の両方を定義します。

```yaml
# application.yaml (抜粋)
surfin:
  adapter:
    # モックサーバーとして機能するWeb Proxy
    openmeteo_mock_server:
      type: "MOCK_SERVER"
      mock_status: 200
      mock_response: "{\"message\": \"Mocked weather data\"}"

    # APIKEY認証を行うWeb Proxy
    openmeteo_api_key_adapter:
      type: "APIKEY"
      key: "your-api-key-value"
      placement: "header"
      key_name: "X-API-Key"

    # HMAC認証を行うWeb Proxy (例)
    openmeteo_hmac_adapter:
      type: "HMAC"
      algorithm: "RSASSA-PSS"
      private_key: "${OPENMETEO_HMAC_PRIVATE_KEY}"
      public_key_id: "some-key-id"
      region: "us-east-1"
```

#### 4.2. `job.yaml` の設定例

`job.yaml` では、`ItemReader` の `web_proxy_ref` で認証情報を付与するアダプターを指定し、`api_url` でモックサーバーの URL をジョブパラメータ経由で指定します。

```yaml
# example/webapi_proxy/cmd/webapi_proxy/resources/job.yaml
job:
  id: "webApiProxyAuthTestJob"
  name: "webApiProxyAuthTestJob"
  steps:
    - id: "readApiStep"
      tasklet:
        reader:
          ref: "mockApiReader" # または実際のビジネスロジックを持つItemReader
          properties:
            name: "myAuthTestReader"
            api_url: "#{jobParameters['mockServerUrl']}" # モックサーバーのURLをジョブパラメータから取得
            max_reads: 1
            web_proxy_ref: "openmeteo_api_key_adapter" # APIKEY認証を行うアダプターを参照
            # または web_proxy_ref: "openmeteo_hmac_adapter" でHMAC認証をテスト
        processor:
          ref: "noopItemProcessor"
        writer:
          ref: "noopItemWriter"
```

#### 4.3. `main.go` での連携

`main.go` では、アプリケーション起動時に `WebProxyProvider` から `type: "MOCK_SERVER"` の Web Proxy Connection を取得し、そのモックサーバーの URL をジョブパラメータとしてジョブに渡します。

```go
// main.go (抜粋)
func main() {
	app := fx.New(
		fx.Options(
			// ... 既存のモジュール ...
			webproxy.Module,
			fx.Invoke(func(jobFactory *config_support.JobFactory) {
				jobFactory.RegisterComponentBuilder("mockApiReader", itemreader.MockAPIReaderBuilder)
			}),
		),
		fx.Invoke(func() {
			jsl.RegisterJobDefinitionFromYAML("webApiProxyAuthTestJob", "example/webapi_proxy/cmd/webapi_proxy/resources/job.yaml")
		}),
		fx.Invoke(func(lifecycle fx.Lifecycle, jobFactory *config_support.JobFactory, webProxyProvider *webproxy.WebProxyProvider) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					// "openmeteo_mock_server" は application.yaml で定義したモックアダプター名
					mockConn, err := webProxyProvider.GetConnection("openmeteo_mock_server")
					if err != nil {
						return fmt.Errorf("failed to get mock web proxy connection: %w", err)
					}
					wpMockConn, ok := mockConn.(*webproxy.WebProxyConnection)
					if !ok {
						return fmt.Errorf("mock connection is not a WebProxyConnection")
					}
					mockServerURL := wpMockConn.GetMockServerURL() // WebProxyConnection に追加されるメソッド

					logger.Infof("Mock API Server URL: %s", mockServerURL)

					jobName := "webApiProxyAuthTestJob"
					job, err := jobFactory.CreateJob(jobName)
					// ... (ジョブ実行ロジック) ...

					jobParams := model.NewJobParameters()
					jobParams.Set("mockServerUrl", mockServerURL+"/api/v1/weather") // モックサーバーのURLを渡す
					// ...
					return nil
				},
				// ... OnStop ロジック ...
			})
		}),
	)
	// ... (アプリケーションの起動と停止) ...
}
```

### 5. OpenAPI/Swagger との連携について

ここまでの実装は、Web Proxy Adapter が内部でローカルモックサーバーを起動し、その URL を提供する基盤を整えるものです。しかし、このモックサーバーが OpenAPI/Swagger の設定を読み込んで動的に振る舞う機能は含まれていません。

OpenAPI/Swagger の設定を読み込み、リクエストのマッチングやレスポンスの動的生成を行うには、`httptest.Server` のハンドラ関数内で、OpenAPI/Swagger 仕様のパース、リクエストのルーティング、スキーマに基づいたレスポンス生成などのロジックを別途実装する必要があります。これは、将来的な機能拡張として検討されるべきです。
