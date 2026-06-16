<p align="center">
  <img src="docs/images/surfin-logo.png" alt="Surfin Logo" width="150"/>
</p>

# 🌊 Surfin - Restartable Batch Processing Framework for Go

[![GoDoc](https://pkg.go.dev/badge/github.com/tigerroll/surfin.svg)](https://pkg.go.dev/github.com/tigerroll/surfin)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/tigerroll/surfin/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/tigerroll/surfin)](https://goreportcard.com/report/github.com/tigerroll/surfin)

[English](./README.md) | 日本語

処理が途中で中断しても、最初からやり直す必要はありません。

担当者が変わっても、持続可能な保守運用を実現します。

**Surfin は、Go 向けのエンタープライズバッチフレームワークです。**

API 連携・ETL・データ同期・レポート生成・データレイク投入など、
大量データ処理に必要な運用機能を標準で提供します。

```
External API → CSV Stream → Transform → Database → Parquet → Data Lake
```

あなたは **「何を処理するか」** に集中してください。

**「どう安全に処理するか」** は、Surfin が担います。

## 😱 Have you ever faced these challenges ?

* バッチが途中で落ちた。どこまで処理したか、誰も知らない。
* とりあえず最初から流し直した。翌朝、データが二重になっていた。
* 再実行フラグ用のテーブルを作ったが、仕様を知っているのは退職した人だけだった。
* 処理済みかどうかを判定するロジックが、バッチごとに微妙に違う。
* 「冪等（べきとう）にしておけばいい」と理想を説かれるが、実装コストが高すぎて断念した。
* 障害が起きるたびに、「どこから再開するか」を会議している。
* バッチの担当者が異動・退職し、誰も全体像を説明できない。

**もし 1 つでも心当たりがあるなら、Surfin はあなたのためのフレームワークです。**

## 🌊 Why Surfin Was Born.

> 私は、担当者不在となった Go 製バッチシステムを保守することになりました。
> LLM にコードを読ませれば、処理内容は瞬時に解析できるでしょう。しかし、運用現場の過酷な現実は「コードの理解」だけでは解決できませんでした。
> 
> * 途中で停止したバッチを、LLM が「安全に再開できる」と保証してくれるわけではない。
> * 障害のたびに「LLM に仕様を聞く」ことが、深夜の復旧作業の代替にはならない。
> * 運用ノウハウとは、コードに埋もれたロジックではなく、**障害発生時に迷わず再開できる「運用設計という名の規律」** である。
> 
> 私が必要だったのは、コードの読み書きを補助するツールではありません。
> 「誰がバトンを渡されても、障害時に迷わず、安全に再開できる仕組み」そのものでした。
>
> その答えが、Surfin です。

Go でバッチを書くこと自体は、驚くほど簡単です。

```go
rows, err := db.QueryContext(ctx, "SELECT * FROM orders WHERE status = 'pending'")
for rows.Next() {
    var order Order
    rows.Scan(&order)
    process(order)
}
```

しかし、**本当の辛さは、その先にあります。**

* プロセスが死んだとき、どこから再開するか？
* 二重起動をどう防ぎ、データの整合性をどう担保するか？
* 100 万件の処理中、5 万件目で起きたエラーだけをどうスキップするか？
* アラートが鳴った深夜、運用者は「次に何をすべきか」を即座に判断できるか？
* そして数年後、このコードの「お作法」を知らない誰かが、安心して保守できるか？

これらに対する答えが、Surfin です。

再実行性、チェックポイント、リトライ制御、ジョブ管理。
これらは単なる「便利な機能」ではありません。**先人たちが現場での泥臭い失敗を繰り返した末に手に入れた、「運用における知恵」そのものです。**

私たちは、その知恵を現代の Go 開発に持ち込みます。

**過去の失敗を、次の開発者の「武器」に変えるために。**

## 🚀 Getting Started with Surfin

パイプラインは YAML で定義します。

```yaml
jobs:
  - name: daily-report
    steps:
      - name: import-report
        reader:
          type: csv-stream
        processor:
          bean: transformReport
        writer:
          type: parquet

```

ビジネスロジックは Go で実装します。

```go
func (p *ReportProcessor) Process(
    ctx context.Context,
    item Report,
) (ReportRecord, error) {
    return transform(item), nil
}

```

処理フローとビジネスロジックは分離されます。

フローを変えるために Go コードを触る必要はありません。

## 📍 Key Problems Solved

**どこまで処理したか分からない**

`JobRepository` と `ExecutionContext` が進捗をチャンク単位で永続化します。

**二重実行が怖い**

同じジョブが二重に起動されても、片方は実行を自動的に拒否します。

**再開地点を管理したくない**

完了済みステップは自動的にスキップされます。失敗したステップだけが再実行されます。

**リトライ処理を毎回書きたくない**

ポリシーとして宣言するだけです。

```yaml
faultTolerance:
  retry:
    maxAttempts: 3
  skip:
    limit: 100

```

## ♻️ Mechanism of Resume

Surfin はチャンクのコミットごとに `ExecutionContext` を DB へ永続化します。再実行時はその位置を復元して、失敗地点から再開します。
Reader の位置保存さえ実装すれば、あとはフレームワークがすべてやります。
失敗した `JobExecution` の検出、コンテキストの復元、完了済みステップのスキップなど、複雑なロジックから解放されます。

## ⚖️ Comparison with Custom Implementation

| 項目 | 自前実装 | Surfin |
| --- | --- | --- |
| 再実行 | 個別実装 | ✅ 標準 |
| Checkpoint | 個別実装 | ✅ 標準 |
| Retry / Skip | 個別実装 | ✅ 標準 |
| OpenTelemetry | 個別実装 | ✅ 標準 |
| ジョブ履歴管理 | 個別実装 | ✅ 標準 |
| 並列実行 | 個別実装 | ✅ 標準 |
| YAML 定義 | なし | ✅ 標準 |

自前で全部作ることは可能です。しかし、再実行性・障害耐性・安全な並行実行が必要になった瞬間、複雑さは爆発します。

**「動いているけど、怖くて触れない」バッチになる前に。**

## 🏗️ Architecture

各責務が明確に分離されています。

```
Job
 └─ Step
      ├─ Reader
      ├─ Processor
      └─ Writer

```

理解しやすく、テストしやすく、保守しやすい設計です。

## 🛠️ Key Features

* **♻️ Restartability**: 失敗地点からの正確な再開。完了済みステップは自動スキップ。
* **📍 Checkpoint**: チャンク単位の進捗を永続化。どこまで処理したかを常に把握。
* **🛡️ Fault Tolerance**: Retry・Skip・Backoff をポリシーとして宣言的に定義。
* **📋 宣言的パイプライン（JSL）**: YAML によるジョブ定義。
* **🔄 トランザクション管理**: `REQUIRED`・`REQUIRES_NEW`・`NESTED` をサポート。
* **✨ Observability**: OpenTelemetry と Prometheus をコアに統合。
* **📈 並列実行・スケーリング**: Split・Decision・Partition・Kubernetes Job をサポート。
* **🔒 楽観的ロック**: ジョブの二重起動を自動検知して実行を拒否。

## 🎯 Use Cases

* **SaaS データ連携**: `Google Ads → CSV → PostgreSQL → Parquet`
* **ETL・データ基盤**: `API → Transform → Iceberg → Analytics`
* **レポート生成**: `Database → Aggregation → CSV / PDF`
* **IoT・工場データ**: `Sensor Data → Batch Processing → Parquet → Data Lake`

## 🐹 Why Go for Batch Processing?

Go はシンプルさを重視する言語です。そのシンプルさは、バッチ処理において最大限に活かされます。

* ⚡ ネイティブの並行性（goroutine）で大量データを効率的に処理
* 📦 シングルバイナリでデプロイが簡単
* 🚀 起動が速く、Kubernetes Job との相性が抜群

Surfin は、過去のエンタープライズシステムで培われた知見を、現代の Go 開発に持ち込みます。

**私たちは過去の知見を捨てません。活かします。**

## 🚀 Getting Started

```bash
go get [github.com/tigerroll/surfin](https://github.com/tigerroll/surfin)
```

👉 **[tutorial - "Hello, Wold!"](docs/tutorial/hello-world.md)**

## 📚 Documentation & Usage

* [イントロダクション・基本概念](https://www.google.com/search?q=./docs/guide/01_introduction.md)
* [セットアップと JSL 定義](https://www.google.com/search?q=./docs/guide/02_setup_and_jsl.md)
* [ステップタイプとコンポーネント](https://www.google.com/search?q=./docs/guide/03_step_types_and_components.md)
* [フォールトトレランスとトランザクション管理](https://www.google.com/search?q=./docs/guide/04_fault_tolerance.md)
* [スケーリングと並列処理](https://www.google.com/search?q=./docs/guide/05_scaling_and_parallelism.md)
* [アーキテクチャと設計原則](https://www.google.com/search?q=./docs/architecture/)
* [実装ロードマップ](https://www.google.com/search?q=./docs/strategy/adapter_and_component_roadmap.md)

## 🆘 Support

質問・バグ報告・機能要望は GitHub Issues へ。

* **GitHub Issues**: [バグ報告・機能要望](https://www.google.com/search?q=https://github.com/tigerroll/surfin/issues)

## 📄 License

MIT
