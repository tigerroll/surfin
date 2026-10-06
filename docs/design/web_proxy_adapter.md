# Web Proxy Adapter 設計

## 1. 目的

Web Proxy Adapterは、Surfinバッチから外部HTTPサービスへアクセスするための通信基盤を提供します。

**Web Proxy Adapterは、HTTP Proxyそのものを提供するコンポーネントではなく、外部HTTPサービスへの通信経路を抽象化するHTTP Infrastructure Adapterです。**

外部サービスとの通信では、以下のような要件が存在します。

* HTTP/HTTPS通信
* HTTP Proxy経由の通信
* TLS設定
* Request Timeout
* ContextによるCancellation
* HTTP Requestへの認証情報の付与
* OAuth2 Tokenの取得・再利用
* Request Signing
* API固有のHeader付与
* RetryやBackoffとの連携

これらのうち、**通信経路と認証・署名などのRequest処理を分離して扱います。**

Web Proxy Adapter自体は特定の外部サービスや認証方式に依存せず、HTTP通信のInfrastructureとして利用できることを目的とします。

---

## 2. 設計方針

### 2.1. 通信経路と認証を分離する

Web Proxy Adapterでは、Proxy RoutingとAuthenticationを同一の責務として扱いません。

```text
                    HTTP Client
                        │
             ┌──────────┴──────────┐
             │                     │
        Request Layer          Transport Layer
             │                     │
       Authentication          Proxy / TLS
       / Signing
             │                     │
             └──────────┬──────────┘
                        │
                     Internet
```

例えば、

```text
Authentication
  ├── API Key
  ├── OAuth2
  ├── Request Signing
  └── None

Transport
  ├── Direct
  └── HTTP Proxy
```

のように、それぞれ独立して差し替えられる構造とします。

これにより、

```text
OAuth2 + Direct
OAuth2 + Proxy
API Key + Direct
Request Signing + Proxy
```

などの組み合わせを自然に扱えます。

---

## 3. Web Proxy Adapterの責務

Web Proxy Adapterが担当するのは、主にHTTP通信のInfrastructureです。

### 3.1. HTTP通信

* HTTP/HTTPS Requestの送信
* ContextによるCancellation
* Timeout
* Connection管理
* TLS設定

### 3.2. Proxy Routing

必要に応じてHTTP Proxyを経由して外部サービスへ接続します。

```text
Application
    │
    ▼
HTTP Client
    │
    ▼
Web Proxy Adapter
    │
    ▼
HTTP Proxy
    │
    ▼
External API
```

Proxyを使用しない場合は、直接外部サービスへ接続します。

```text
Application
    │
    ▼
HTTP Client
    │
    ▼
Web Proxy Adapter
    │
    ▼
External API
```

Proxyの有無はApplicationから利用するHTTP Clientの通信設定として扱い、認証方式とは独立させます。

---

## 4. Authentication / Request Signing

外部サービスによっては、HTTP Requestに認証情報や署名を付与する必要があります。

これはWeb Proxy Adapterそのものの責務ではなく、Requestを処理する認証・署名コンポーネントとして分離します。

概念的には、Requestに対する認証や署名付与を行うメカニズムを想定します。

```text
Request Authentication / Processing
    ├── API Key
    ├── OAuth2
    └── Request Signing
```

これにより、例えばAPI Key認証やOAuth2、あるいは特定の署名方式を必要とするサービスに対して、柔軟にコンポーネントを追加できます。

実際のインターフェースや命名は実装状況に応じて決定します。

---

## 5. OAuth2

OAuth2を利用する外部サービスでは、Token取得とRequestへのToken付与を認証コンポーネントとして扱います。

```text
HTTP Request
     │
     ▼
OAuth2 Authenticator
     │
     ├── Token Cache
     │
     ├── Token Refresh
     │
     ▼
Authorization Header
     │
     ▼
HTTP Client
```

Tokenは必要に応じてキャッシュし、有効期限を考慮して再取得します。

Client SecretなどのCredentialは、Job定義やGit管理されるYAMLへ直接記述することを前提としません。

環境変数、Secret Manager、Workload Identityなど、実行環境が提供するSecret管理機構から注入できる構成を推奨します。

---

## 6. Request Signing

一部の外部サービスでは、Request内容から署名を生成して認証する必要があります。

このような署名処理もWeb Proxy Adapterから分離します。

```text
HTTP Request
     │
     ▼
Request Signer
     │
     ├── Canonical Request
     ├── Signature
     └── Authentication Headers
     │
     ▼
HTTP Client
```

署名方式は外部サービスごとに異なるため、Web Proxy Adapterに特定の署名方式を組み込まないことを原則とします。

例えばAWS系サービスなど、特定のRequest Signing方式を必要とするサービスでは、サービス固有Adapterから利用します。

---

## 7. Configuration

Web Proxy AdapterのConfigurationは、HTTP通信に必要な設定に限定します。

例えば、

```go
type WebProxyConfig struct {
    URL        string `yaml:"url"`
    Timeout    time.Duration `yaml:"timeout"`
    TLS        TLSConfig `yaml:"tls"`
}
```

など、Proxy / Transportに関係する設定を保持します。

Proxy認証が必要な環境では、Proxy Credentialについても実行環境から安全に注入できる仕組みを利用します。

OAuth2 Client ID、Client Secret、Private Keyなどの外部サービスCredentialは、Web Proxy Configurationに直接含めません。

---

## 8. API EndpointはApplication側で管理する

外部APIのEndpointは、Web Proxy AdapterのConfigurationとして扱いません。

例えば、

```text
Web Proxy Adapter
        │
        │ HTTP transport
        ▼
Amazon Adapter
        │
        │ API Endpoint
        ▼
Amazon SP-API
```

のように、Endpointは外部サービスAdapterまたはApplicationの責務とします。

これにより、Web Proxy AdapterはAmazon、GitHub、Slack、その他のHTTP APIに依存しない汎用Infrastructureになります。

---

## 9. 外部サービスAdapterとの関係

外部サービス固有の処理は、Web Proxy Adapterの上位に配置します。

例えばAmazon Seller Partner APIの場合、

```text
Surfin Job
    │
    ▼
Amazon Adapter (業務ロジック / Report Lifecycle)
    │
    ▼
Authenticator (認証 / Request Signing)
    │
    ▼
Web Proxy Adapter (HTTP Transport / Proxy)
    │
    ▼
Amazon SP-API
```

となります。

Amazon固有のAPI仕様、Report生成、Polling、Document取得などはAmazon Adapterの責務です。
認証・署名はAuthenticatorの責務です。
Web Proxy Adapterは、それらのHTTP通信を実行するための共通Infrastructureとして利用します。

---

## 10. Retryとの関係

RetryはWeb Proxy Adapterに閉じ込めません。

外部API通信では、

* Connection Error
* Timeout
* HTTP 429
* HTTP 5xx
* Authentication Error
* Application Error

など、異なる種類のエラーが発生します。

そのため、Retryの判断は呼び出し側の外部サービスAdapterやSurfinのExecution Semanticsと連携して決定します。

概念的には、

```text
ChunkStep / Tasklet
        │
        ▼
External Service Adapter
        │
        ▼
HTTP Client
        │
        ▼
Web Proxy Adapter
```

とし、Web Proxy Adapter自身が無条件にRetryを行うことは避けます。

特に、HTTP Requestが副作用を持つ場合には、Retry可能性をHTTP Transportだけから判断することはできません。

---

## 11. ContextとCancellation

HTTP通信では `context.Context` を利用して、

* Job cancellation
* Step cancellation
* Timeout
* Shutdown

などをHTTP Requestへ伝播させます。

```go
req, err := http.NewRequestWithContext(
    ctx,
    http.MethodGet,
    endpoint,
    body,
)
```

これにより、バッチの停止やCancellationに伴って、外部HTTP通信も適切に終了させることができます。

Contextは通信制御のために利用し、CredentialやConnectionなどの長寿命ResourceをContextへ格納することは原則として行いません。

---

## 12. Resource Lifecycle

Web Proxy Adapterが管理するHTTP ClientやTransportなどのResourceについては、明確なLifecycleを持たせます。

```text
Create
  │
  ▼
Use
  │
  ▼
Close
```

ただし、HTTP ClientのCloseとJobの成功・失敗を同一視しません。

また、HTTP ClientのResource LifecycleとSurfinのDB Transaction Lifecycleも独立しています。

```text
DB Transaction
    ├── Begin
    ├── Commit
    └── Rollback

HTTP Resource
    ├── Create
    ├── Use
    └── Close
```

---

## 13. Execution Semanticsとの関係

Web Proxy Adapterは通信Infrastructureであり、バッチの成功・失敗そのものを定義するものではありません。

例えば、

```text
HTTP Request
    │
    ▼
Timeout
```

が発生した場合、

```text
HTTP通信の失敗
        ↓
External Service Adapter
        ↓
Retry可能か判断
        ↓
Surfin Execution Semantics
        ↓
Retry / Fail / Restart
```

という形で処理します。

これにより、通信エラーとBatch Executionの状態を混同しません。

---

## 14. 設計上の原則

Web Proxy Adapterでは以下を原則とします。

1. **HTTP通信と外部サービスの業務ロジックを分離する**
2. **Proxy RoutingとAuthenticationを分離する**
3. **OAuth2やRequest SigningをWeb Proxy Adapter固有の責務にしない**
4. **API EndpointをWeb Proxy Configurationに含めない**
5. **CredentialをJob YAMLなどへ直接記述しない**
6. **HTTP Client自身が無条件にRetryしない**
7. **ContextはCancellation、Timeout、Request Scopeの伝播に利用する**
8. **HTTP Resource LifecycleとDB Transaction Lifecycleを分離する**
9. **外部サービス固有の認証・署名はAdapter側で扱う**
10. **通信エラーとBatch Executionの成功・失敗を分離する**
11. **外部サービスの非同期処理は、そのサービス固有のExecutionとして扱う**

---

## 15. Amazon Seller Reportへの適用例

Amazon Seller Reportを取得するApplicationでは、以下のように責務を分離して利用できます。

```text
                  Surfin Job
                      │
                      ▼
              Amazon Report Adapter
        (Report Semantics / Lifecycle)
                      │
                      ▼
                Authenticator
           (Auth / Request Signing)
                      │
                      ▼
              Web Proxy Adapter
            (HTTP Transport / Proxy)
                      │
                      ▼
                Amazon SP-API
                      │
                      ▼
                Report Document
                      │
                      ▼
                   Stream
                      │
                      ▼
                 ChunkStep
                      │
                      ▼
                ParquetWriter
                      │
                 ┌────┴────┐
                 │         │
                S3        GCS
```

この構造により、Amazon固有の処理、認証処理、Surfin共通のHTTP Infrastructure、さらにS3/GCSなどのStorage Infrastructureをそれぞれ独立して扱うことができます。

---

## 16. 今後の拡張

Web Proxy Adapterは、特定の認証方式や外部サービスに依存しないことを基本とします。

今後必要に応じて、

* HTTP Proxy
* TLS / mTLS
* OAuth2
* API Key
* Request Signing
* Custom Headers
* Connection Pooling
* HTTP Transport設定

などを組み合わせて利用できる構造を維持します。

一方、外部サービス固有の認証方式やAPI仕様については、各サービスAdapter側に実装します。

これにより、SurfinのHTTP通信基盤を汎用化しながら、外部サービスごとの特殊な仕様がフレームワーク本体へ侵食することを防ぎます。
