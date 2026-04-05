# Web Proxy Adapter 方式設計書 (surfin アプリケーション構成への統合版)

### 1. 目的

`surfin` のバッチ処理において、外部 Web API との通信における認証（HMAC 署名、OAuth2、API Key 等）の複雑なロジックをインフラ層に集約し、ビジネスロジック（`ItemReader` 等のコンポーネント）が本来の業務仕様（リクエスト構築）に専念できる環境を提供します。これにより、認証ロジックの再利用性、保守性、およびセキュリティを向上させます。

### 2. 設計の基本方針

*   **関心の分離:** 認証の「計算・付与」は Adapter が担い、「正規化・仕様定義」は Component が担います。
*   **透過的 Proxy:** `ItemReader` は標準的な HTTP Client を通じて通信を行い、背後で Proxy がリクエストを自動加工（上書き）します。
*   **設定駆動:** 認証方式や鍵情報は `pkg/batch/core/config/config.go` で定義される `Config` 構造体内の `SurfinConfig.AdapterConfigs` を通じて `application.yaml` と環境変数により、コードの変更なしで切り替え可能とします。
*   **既存リソース管理の活用:** `pkg/batch/core/adapter/interfaces.go` で定義されている `ResourceProvider` および `ResourceConnection` のパターンに準拠し、Web Proxy を `surfin` のリソースとして管理します。

### 3. アーキテクチャ構成

| レイヤー | コンポーネント | 責務 |
| :--- | :--- | :--- |
| **Component** | `ItemReader` (例: `amazonpay_v2.go`) | ・エンドポイントの決定<br>・リクエストの正規化（Canonical String 構築）<br>・`WebProxyConnection` から取得した HTTP Client を使用した通信<br>・Context 経由での署名指示 (HMAC の場合) |
| **Adapter** | `WebProxyConnection` (新設) | ・`adapter.ResourceConnection` インターフェースを実装<br>・秘密鍵/トークンの管理<br>・署名アルゴリズム（RSASSA-PSS 等）の実行<br>・HTTP ヘッダーへの認証情報注入<br>・内部に `http.Client` を持ち、`http.RoundTripper` を実装してリクエストを加工 |
| **Adapter** | `WebProxyProvider` (新設) | ・`adapter.ResourceProvider` インターフェースを実装<br>・`WebProxyConfig` に基づく `WebProxyConnection` インスタンスの生成と管理<br>・`JobFactory` の `resourceProviders` マップを通じて提供される |
| **Configuration** | `WebProxyConfig` (新設) | ・`pkg/batch/adapter/webproxy/config/config.go` に定義される Web Proxy の設定スキーマ |
| **Core** | `JobFactory` | ・`pkg/batch/core/config/support/jobfactory.go` にて `WebProxyProvider` を `resourceProviders` として管理し、コンポーネントに注入可能にする |

### 4. 主要な認証方式と実装仕様

`WebProxyConnection` の内部実装として、以下の認証方式をサポートします。

#### A. HMAC / Signature (Amazon Pay v2 等)

*   **概要:** リクエスト内容に基づいた動的な署名。
*   **実装:**
    1.  Component (`ItemReader`) が署名対象文字列を作成し、`context.Context` に格納して `http.Request` に渡します。
    2.  `WebProxyConnection` は、内部の `http.RoundTripper` 実装において、`context.Context` から署名対象文字列を読み取ります。
    3.  `WebProxyConnection` は、設定された鍵情報とアルゴリズム（RSASSA-PSS 等）を用いて署名を生成します。
    4.  `WebProxyConnection` は、生成した署名を `Authorization` ヘッダー等の指定箇所に注入（パッチ）します。

#### B. OAuth2 Bearer

*   **概要:** 期間限定トークンによる認可。
*   **実装:**
    1.  `WebProxyConnection` は、トークンの有効期限を管理（キャッシュ）します。
    2.  必要に応じて Client Credentials 等のフローでトークンを自動更新（リフレッシュ）します。
    3.  `Authorization: Bearer <token>` ヘッダーを自動付与します。

#### C. API Key

*   **概要:** 固定のキーによる認証。
*   **実装:**
    1.  `WebProxyConnection` は、`WebProxyConfig` の設定に基づき、Header、Query、または Auth Header にキーを注入します。

### 5. データフロー（HMAC の例）

1.  **Component (`ItemReader`):**
    *   外部 API への `http.Request` を作成します。
    *   HMAC 署名に必要な署名対象文字列を計算し、`context.Context` にセットします。
    *   `WebProxyProvider` から取得した `WebProxyConnection` を通じて得られる `http.Client` の `Do(req)` メソッドを実行します。
2.  **`WebProxyConnection` (内部の `http.RoundTripper`):**
    *   `http.Request` を受け取ります。
    *   `req.Context()` から署名対象文字列を読み取ります。
    *   自身の設定（`WebProxyConfig`）から鍵情報を取得し、署名対象文字列と組み合わせて署名を計算します。
    *   計算した署名を `req.Header` に追加または上書きします（例: `Authorization` ヘッダー）。
    *   加工済みの `http.Request` を実際のネットワークに送信します。
3.  **External API:**
    *   加工済みのリクエストを受信し、認証・認可を行います。

### 6. 設定スキーマ案 (`application.yaml` および `pkg/batch/adapter/webproxy/config/config.go`)

`pkg/batch/adapter/webproxy/config/config.go` に `WebProxyConfig` を定義します。

```go
// pkg/batch/adapter/webproxy/config/config.go

// WebProxyConfig は Web Proxy の設定を定義します。
type WebProxyConfig struct {
	Type         string `yaml:"type"`             // 認証タイプ (例: "HMAC", "OAUTH2", "APIKEY", "MOCK_SERVER", "NONE")
	Algorithm    string `yaml:"algorithm,omitempty"` // HMAC の署名アルゴリズム (例: "RSASSA-PSS")
	PrivateKey   string `yaml:"private_key,omitempty"` // HMAC の秘密鍵 (環境変数から読み込む)
	PublicKeyId  string `yaml:"public_key_id,omitempty"` // HMAC の公開鍵ID
	Region       string `yaml:"region,omitempty"` // HMAC のリージョン
	GrantType    string `yaml:"grant_type,omitempty"` // OAuth2 のグラントタイプ (例: "client_credentials")
	ClientId     string `yaml:"client_id,omitempty"` // OAuth2 のクライアントID
	ClientSecret string `yaml:"client_secret,omitempty"` // OAuth2 のクライアントシークレット
	TokenUrl     string `yaml:"token_url,omitempty"` // OAuth2 のトークンエンドポイントURL
	Key          string `yaml:"key,omitempty"`           // API Key の値
	Placement    string `yaml:"placement,omitempty"`     // API Key の配置場所 ("header", "query", or "auth_header")
	KeyName      string `yaml:"key_name,omitempty"`      // API Key のヘッダー名またはクエリパラメータ名
	APIEndpoint  string `yaml:"api_endpoint,omitempty"`  // プロキシするAPIのエンドポイントURL
	MockResponse string `yaml:"mock_response,omitempty"` // MOCK_SERVER 用の固定レスポンスボディ
	MockStatus   int    `yaml:"mock_status,omitempty"`   // MOCK_SERVER 用の固定HTTPステータスコード
}

// pkg/batch/core/config/config.go
// SurfinConfig はアプリケーション全体の構成を定義します。
type SurfinConfig struct {
	// ... 既存のフィールド ...
	// AdapterConfigs は様々なアダプターの設定を保持します。
	// ここに WebProxyConfig を含めることができます。
	AdapterConfigs map[string]interface{} `yaml:"adapter"` // 既存の AdapterConfigs フィールドを使用
}
```

`application.yaml` の例:

```yaml
# application.yaml
surfin:
  # ... 既存の設定 ...
  adapter: # 既存の adapter キーを使用
    amazonPayProxy:
      type: "HMAC"
      algorithm: "RSASSA-PSS"
      private_key: "${AMAZON_PAY_PRIVATE_KEY}" # 環境変数から
      public_key_id: "${AMAZON_PAY_PUBLIC_KEY_ID}"
      region: "jp"

    githubApiProxy:
      type: "OAUTH2"
      grant_type: "client_credentials"
      client_id: "${GH_CLIENT_ID}"
      client_secret: "${GH_CLIENT_SECRET}"
      token_url: "https://github.com/login/oauth/access_token"

    simpleSaaSProxy:
      type: "APIKEY"
      key: "${SAAS_API_KEY}"
      placement: "header" # header, query, or auth_header
      key_name: "X-API-KEY"
```

### 7. 設計のメリット

*   **セキュリティ:** 秘密鍵や API Key を扱うロジックを `WebProxyConnection` に集約し、`surfin` の設定メカニズム（環境変数からの読み込み、`GetMaskedParameterKeys` によるマスク処理など）と連携させることで、露出リスクを低減します。
*   **保守性:** 外部サービスの認証仕様変更に対し、`WebProxyConnection` の実装のみを修正すればよく、ビジネスロジック（`ItemReader`）に影響を与えません。
*   **再利用性:** `surfin` の既存の `adapter.ResourceProvider` パターンに則ることで、`WebProxyProvider` を `JobFactory` 経由で任意のコンポーネントに注入可能となり、将来のあらゆる外部連携に汎用的な認証基盤を提供します。
*   **設定の柔軟性:** `application.yaml` と環境変数を通じて、コードの変更なしに認証方式やパラメータを切り替えることが可能です。
