# Web Proxy Adapter 設計

## 1. 目的
Web Proxy Adapter は、外部APIとの通信において、認証（HMAC署名、OAuth2）やプロキシ経由のルーティングを抽象化し、バッチジョブが安全かつ透過的に外部サービスと連携できるようにするためのコンポーネントです。

## 2. 構造体定義
`WebProxyConfig` は、プロキシの認証方式や接続情報を保持します。

```go
type WebProxyConfig struct {
	Type         string `yaml:"type"`         // "hmac", "oauth2", "apikey", "MOCK_SERVER"
	Algorithm    string `yaml:"algorithm"`    // HMAC署名アルゴリズム
	PrivateKey   string `yaml:"private_key"`  // HMAC秘密鍵
	PublicKeyId  string `yaml:"public_key_id"`// HMAC公開鍵ID
	Region       string `yaml:"region"`       // リージョン
	GrantType    string `yaml:"grant_type"`   // OAuth2 Grant Type
	ClientId     string `yaml:"client_id"`    // OAuth2 Client ID
	ClientSecret string `yaml:"client_secret"`// OAuth2 Client Secret
	TokenUrl     string `yaml:"token_url"`    // OAuth2 Token URL
	MockResponse string `yaml:"mock_response,omitempty"` // MOCK_SERVER 用の固定レスポンス
	MockStatus   int    `yaml:"mock_status,omitempty"`   // MOCK_SERVER 用の固定ステータス
}
```

## 3. 主要機能
* **認証の抽象化**: `WebProxyAdapter` インターフェースを通じて、呼び出し元は認証の詳細を意識せずにリクエストを送信可能。
* **セキュリティ**: 秘密鍵やクライアントシークレットを安全に管理し、リクエストヘッダーに署名を付与。
* **プロキシルーティング**: ネットワーク構成に応じたプロキシ設定の適用。
* **モックサーバー機能**: 外部API依存なしでのテスト環境構築。`MOCK_SERVER` タイプを指定することで、`httptest.Server` を利用してローカルでモックサーバーを起動し、認証ロジックの検証をローカルで完結させます。

## 4. 考慮事項
* **鍵管理**: 秘密鍵は環境変数やシークレット管理サービスから注入することを推奨。
* **トークンキャッシュ**: OAuth2トークンは有効期限までキャッシュし、API呼び出しのオーバーヘッドを削減する。
