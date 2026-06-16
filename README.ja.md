<p align="center">
  <img src="docs/images/surfin-logo.png" alt="Surfin Logo" width="150"/>
</p>

# 🌊 Surfin - Batch framework

[![GoDoc](https://pkg.go.dev/badge/github.com/tigerroll/surfin.svg)](https://pkg.go.dev/github.com/tigerroll/surfin) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/tigerroll/surfin/blob/main/LICENSE) [![Go Report Card](https://goreportcard.com/badge/github.com/tigerroll/surfin)](https://goreportcard.com/report/github.com/tigerroll/surfin)

[English](./README.md) | 日本語

堅牢なバッチアプリケーションの開発を可能にするために設計された、軽量で包括的な Go 向けのバッチフレームワークです。

Surfin は、大量のレコード処理に必要不可欠な再利用可能機能を提供します。これには、ロギング/トレーシング、トランザクション管理、ジョブ処理の統計情報、ジョブのリスタート、スキップ、およびリソース管理が含まれます。さらに、最適化およびパーティショニング技術を通じて、極めて大容量かつ高パフォーマンスなバッチジョブを可能にする、より高度な技術的サービスや機能も提供しています。シンプルなバッチジョブから、複雑で大容量なバッチジョブにいたるまで、このフレームワークを活用することで、非常に高い拡張性（スケーラビリティ）を持って膨大なデータ量を処理することができます。

## Restartable Batch Processing Framework for Go

もう処理が途中で中断しても、最初からやり直す必要はありません。JSR352 のナレッジを活用し、持続可能な保守運用を実現します。

### 😱 Have you ever faced these challenges ?

**もし 1 つでも心当たりがあるなら、Surfin はあなたのためのフレームワークです。**

- バッチが途中で落ちた。どこまで処理したか、誰も知らない。
- とりあえず最初から流し直した。翌朝、データが二重になっていた。
- 再実行フラグ用のテーブルを作ったが、仕様を知っているのは退職した人だけだった。
- 処理済みかどうかを判定するロジックが、バッチごとに微妙に違う。
- 「冪等（べきとう）にしておけばいい」と理想を説かれるが、実装コストが高すぎて断念した。
- 障害が起きるたびに、「どこから再開するか」を会議している。
- バッチの担当者が異動・退職し、誰も全体像を説明できない。

### 🎯 Use Cases

**API 連携・ETL・データ同期・レポート生成・データレイク投入など、大量データ処理に必要な運用機能を標準で提供します。**

- **SaaS データ連携**: `External API → CSV Stream → Transform → Database → Parquet → Data Lake`
- **ETL・データ基盤**: `API → Transform → Iceberg → Analytics`
- **レポート生成**: `Database → Aggregation → CSV / PDF`
- **IoT・工場データ**: `Sensor Data → Batch Processing → Parquet → Data Lake`

あなたは **「何を処理するか」** に集中してください。 **「どう安全に処理するか」** は、Surfin が担います。

## 🐹 Motivation: Why Surfin?

### Go 製バッチシステムが抱える「ジレンマ」

Go はシンプルさを重視する言語です。そのシンプルさは、バッチ処理において最大限に活かされます。

* ⚡ ネイティブの並行性（goroutine）で大量データを効率的に処理
* 📦 シングルバイナリでデプロイが簡単
* 🚀 起動が速く、Kubernetes Job との相性が抜群

現代のインフラストラクチャにおいて、バッチ処理に Go を採用するメリットは圧倒的です。メモリフットプリントが極めて小さく、起動はミリ秒単位。単一のバイナリ（シングルバイナリ）として動作するポータビリティの高さは、AWS ECS Tasks、Kubernetes Jobs、サーバーレス（FaaS）のような、短寿命でクラウドネイティブな実行環境と最高にマッチします。さらに、Go で書かれた Web API のコード（DB スキーマやドメインロジック）を、そのまま 100% 共通アセットとして再利用できるという強力な利点もあります。
<br/> しかし、この「シンプルさ」という甘い罠の先に、深刻な落とし穴が待っています。

標準ライブラリの `for rows.Next()` ループを使って、正常系（ハッピーパス）の実装をシュッと書き上げるのは驚くほど簡単です。また、強力な並行処理（Goroutine）を使って「爆速で処理するコード」も、以下のように数行で書けてしまいます。

```go
// 準標準の errgroup を使った、一見スマートな並行処理の例
eg, ctx := errgroup.WithContext(ctx)

for _, item := range items {
    item := item
    eg.Go(func() error {
        // 1件でもエラーが出ると Context がキャンセルされ、
        // 処理中の他のアイテムもすべて道連れに止まってしまう（一蓮托生の罠）
        return process(ctx, item) 
    })
}

if err := eg.Wait(); err != nil {
    return err
}

```

しかし、本当の地獄は、この「シンプルさ」の先に待っています。
いざ本番運用に耐えうるバッチアーキテクチャを構築しようとした瞬間、開発者は分散システムにおける無数の泥臭い課題に、1から自前で立ち向かうことになります。

* **障害復旧（レジリエンス）：** 上記のコードで 100 万件の処理中、5 万件目でプロセスが異常終了したとき、データの重複や二重実行を起こさず、どう安全にリトライ（冪等性の担保）するか？
* **一蓮托生の回避：** 壊れたデータを1件だけスキップして残りの99万9999件の処理を継続したいとき、スレッドセーフを保ちながら、どうやって「エラー上限数（全体の1%を超えたら全停止など）」の高度な制御を組み込むか？
* **分散ロックの不完全さ：** 複数プロセスの二重起動を厳格に防ぎつつ、インフラ側からコンテナが強制終了（`SIGKILL`）された際に発生する「解除されないロック（疑似生存）」をどうハンドリングするか？
* **深夜の意思決定（可観測性）：** 午前3時にアラートが鳴り響いたとき、オンコール担当者はログだけを見て、影響範囲の特定と「次に何をすべきか」を即座に判断できるか？

Surfin は、 **過去のエンタープライズシステムで培われた 知見を、現代の Go 開発に持ち込み活かします。**

## 🚀 Getting Started with Surfin

インストールはとても簡単です。

```bash
go get github.com/tigerroll/surfin
```

シンプルなジョブは、最小限のYAMLだけで定義できます。

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

#### より実践的なJSL（Job Specification Language）の例

ステップ間のトランジション、アイテム単位のリトライ・スキップポリシー、チャンクサイズなども、すべてYAMLで宣言できます。

```yaml
id: myJob
name: サンプルジョブ

flow:
  start-element: extractStep
  elements:
    extractStep:
      id: extractStep
      chunk:
        reader:
          ref: myItemReader
        processor:
          ref: myItemProcessor
        writer:
          ref: myItemWriter
        chunk-size: 100
        item-retry:
          max_attempts: 3
          initial_interval: 1s
        item-skip:
          skip_limit: 10
      transitions:
        - on: COMPLETED
          to: notifyStep
        - on: FAILED
          fail: true

    notifyStep:
      id: notifyStep
      tasklet:
        ref: notifyTasklet
      transitions:
        - on: COMPLETED
          end: true
```

ジョブの構造（Job → Step → Chunk）と、フォールトトレランス（Retry/Skip）の設定が、コードを書かずに表現されています。

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

実装者がやることは、Readerに現在位置を保存・復元するロジックを書くことだけです。

```go
// Readerが現在位置をExecutionContextに保存する
func (r *MyReader) Update(ctx context.Context, ec *model.ExecutionContext) error {
    ec.PutInt("read.offset", r.currentOffset)
    return nil
}

// 再実行時のOpenで位置を復元する
func (r *MyReader) Open(ctx context.Context, ec *model.ExecutionContext) error {
    if offset, ok := ec.GetInt("read.offset"); ok {
        r.currentOffset = offset
    }
    return nil
}
```

あとはフレームワークがすべてやります。
失敗した `JobExecution` の検出、コンテキストの復元、完了済みステップのスキップなど、複雑なロジックから解放されます。

## ⚖️ Comparison with Existing Solutions

自前で全部作ることは可能です。
しかし、再実行性・障害耐性・安全な並行実行が必要になった瞬間、複雑さは爆発します。

**「動いているけど、怖くて触れない」バッチになる前に。**

バッチ処理の複雑な要件を自前で実装し続けることは、メンテナンスコストの増大を招きます。
JSR352 は Java エコシステムにおける標準ですが、Go で同様の堅牢性を求める場合、Surfin がその答えとなります。

| Feature                | 自前実装 (Go) | JSR352 (Java)   | Surfin (Go)  |
| ---------------------- | --------- | --------------- | ------------ |
| Chunk-based Processing | カスタム実装    | ✅ 標準            | ✅ 標準         |
| Restartability         | カスタム実装    | ✅ 標準            | ✅ 標準         |
| Fault Tolerance        | カスタム実装    | ✅ 標準            | ✅ 標準         |
| Declarative I/O        | カスタム実装    | ✅ 標準            | ✅ 標準         |
| Transaction Management | カスタム実装    | ✅ 標準            | ✅ 標準         |
| Observability          | カスタム実装    | ✅ 標準            | ✅ 標準         |
| Parallel Execution     | カスタム実装    | ✅ 標準            | ✅ 標準         |
| Job Control            | カスタム実装    | ✅ 標準            | ✅ 標準         |
| Definition Method      | コード記述     | XML/Java Config | ✅ YAML (JSL) |

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

- **📦 Chunk-based Processing**: チャンク単位の処理とチェックポイントによる進捗管理。
- **♻️ Restartability**: 失敗地点からの正確な再開。完了済みステップは自動スキップ。
- **🛡️ Fault Tolerance**: Retry・Skip・Backoff をポリシーとして宣言的に定義。
- **📋 Declarative I/O & Pipeline**: YAML (JSL) によるジョブ定義と、Reader/Writerの宣言的な分離。
- **🔄 Transaction Management**: `REQUIRED`・`REQUIRES_NEW` 等をサポートした堅牢なトランザクション管理。
- **✨ Observability**: OpenTelemetry と Prometheus をコアに統合。
- **📈 Parallel Execution**: Split・Decision・Partition による並列処理とスケーリング。
- **🔒 Job Control**: 楽観的ロックによる二重起動防止と、ジョブのライフサイクル（Start/Stop）管理。

## 📚 Documentation & Usage

👉 **[tutorial - "Hello, World!"](./docs/tutorial/hello-world.md)**

- [イントロダクション・基本概念](./docs/guide/01_introduction.md)
- [セットアップと JSL 定義](./docs/guide/02_setup_and_jsl.md)
- [ステップタイプとコンポーネント](./docs/guide/03_step_types_and_components.md)
- [フォールトトレランスとトランザクション管理](./docs/guide/04_fault_tolerance.md)
- [スケーリングと並列処理](./docs/guide/05_scaling_and_parallelism.md)
- [アーキテクチャと設計原則](./docs/architecture/)
- [実装ロードマップ](./docs/strategy/adapter_and_component_roadmap.md)

## 🆘 Support

質問・バグ報告・機能要望は GitHub Issues へ。

- **GitHub Issues**: [バグ報告・機能要望](https://github.com/tigerroll/surfin/issues)

## 📄 License

MIT
