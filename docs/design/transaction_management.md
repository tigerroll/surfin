# トランザクション管理の設計

## 1. 概要

本ドキュメントは、Surfin バッチフレームワークにおけるトランザクション管理の設計方針を記述します。

バッチ処理では、データベースの整合性を維持するためのトランザクションと、ファイル・オブジェクトストレージなどの非トランザクションリソースを同一視することはできません。

Surfinでは、これらを明確に分離し、

- データベーストランザクション
- リソースライフサイクル
- 生成物（Artifact）の確定

をそれぞれ独立した概念として扱います。

これにより、データベースを利用するバッチだけでなく、S3、GCS、ローカルファイルなどを利用するバッチにも対応しつつ、バッチ実行における成功・失敗・再開の意味論を明確にします。

---

## 2. 設計方針

### 2.1. データベーストランザクションとリソースを分離する

Surfinでは、データベーストランザクションと非データベースリソースを同一の抽象化として扱いません。

```text
Transaction
  ├── Begin
  ├── Commit
  └── Rollback

Resource
  ├── Open
  ├── Write
  ├── Flush
  └── Close
```

データベーストランザクションはACID特性に基づいて処理されます。

一方、S3、GCS、ローカルファイルなどのリソースは、通常、データベーストランザクションと同じ意味でのCommit/Rollbackを提供しません。

したがって、これらを無理に一つのTransactionモデルへ統合しません。

---

## 2.2. トランザクションの抽象化

データベーストランザクションについては、具体的なデータベース実装を実行エンジンから隠蔽します。

トランザクションのライフサイクルは、概念的には以下のように扱います。

```text
Transaction
  ├── Begin
  ├── Commit
  └── Rollback
```

ChunkStepなどの実行エンジンは、具体的なDB実装に直接依存せず、トランザクション境界を管理します。

DB操作そのものは、RepositoryやItemWriterなどのInfrastructure実装が担当します。

---

## 2.3. Contextによるトランザクション伝播

トランザクションは `context.Context` を介して実行コンポーネントへ伝播させます。

ChunkStepなどの実行エンジンは、チャンク処理の開始時にトランザクションを開始し、そのトランザクションをContextへ格納します。

概念的には以下の流れになります。

```text
ChunkStep
    │
    ├── Begin Transaction
    │
    ▼
Context
    │
    ├── Reader
    ├── Processor
    └── Writer
    │
    ▼
Commit / Rollback
```

ItemWriterなど、データベーストランザクションを必要とするコンポーネントはContextからトランザクションを取得して利用できます。

```go
func (w *MyWriter) Write(ctx context.Context, items []MyItem) error {
    tx, ok := tx.TxFromContext(ctx)
    if !ok {
        return fmt.Errorf("transaction not found in context")
    }

    // txを利用したデータベース操作
    return tx.Exec(...)
}
```

ただし、Contextにトランザクションを格納することは、トランザクションそのものをContextの責務にすることを意味しません。

Contextはあくまで実行コンテキストとして伝播に利用し、トランザクションのライフサイクルはChunkStepなどの実行エンジンが管理します。

---

## 3. Chunkにおけるトランザクション境界

ChunkStepでは、基本的にチャンク単位でデータベーストランザクションを管理します。

```text
Begin
  │
  ├── Read
  ├── Process
  ├── Write
  │
  ▼
Commit
```

処理中にエラーが発生した場合は、

```text
Begin
  │
  ├── Read
  ├── Process
  ├── Write
  │
  ├── Error
  │
  ▼
Rollback
```

となります。

これにより、データベースへの書き込みについては、Chunk単位で成功または失敗を定義できます。

このトランザクション境界は、CheckpointやRestartなどのExecution Semanticsとも整合させます。

---

## 4. 非データベースリソースの扱い

S3、GCS、ローカルファイルなどのストレージは、通常、データベースと同じACIDトランザクションを提供しません。

そのため、これらをDBトランザクションへ直接参加させる設計は採用しません。

ストレージなどのリソースは、以下のようなライフサイクルとして扱います。

```text
Open
  │
  ▼
Write
  │
  ▼
Flush
  │
  ▼
Close
```

ここで `Close` や `Flush` は、データベースにおける `Commit` と同じ意味を持ちません。

例えばParquetWriterの `Close` は、Parquetファイルを正常に終了させるためのリソースライフサイクル上の操作です。

これは、バッチ全体またはChunkの業務上の成功を意味するものではありません。

---

## 5. Artifactの確定

ファイルやオブジェクトストレージへ出力する処理では、生成途中のデータと最終的に利用可能なデータを区別します。

例えば、

```text
Processing
    │
    ▼
Temporary Artifact
    │
    ▼
Finalize
    │
    ▼
Final Artifact
```

というライフサイクルを利用できます。

ローカルファイルシステムでは、一時ファイルへの書き込み後にrenameすることで、完成したファイルだけを公開する方式を利用できます。

オブジェクトストレージでは、必要に応じて一時オブジェクトを利用し、処理完了後に最終オブジェクトとして確定する方式を検討します。

これにより、処理途中の不完全なArtifactが後続システムから利用されることを防ぎます。

---

## 6. TransactionとArtifactの関係

データベーストランザクションとArtifactの確定は、同じCommit境界を持つとは限りません。

例えば、

```text
Chunk 1
  │
  ├── DB Transaction
  │      └── Commit
  │
  └── Artifact
         └── Write / Flush
```

のように、それぞれ異なるライフサイクルを持ちます。

したがって、Surfinでは、

```text
DB Transaction
        ≠
Resource Lifecycle
        ≠
Artifact Finalization
```

として扱います。

これは、DBとオブジェクトストレージの間に分散トランザクションを導入することを意味するものではありません。

---

## 7. 外部サービスとの連携

外部APIや外部サービスとの通信も、DBトランザクションとは分離して扱います。

例えばAmazon Seller Partner APIからレポートを取得する場合、

```text
Create Report
     │
     ▼
Report Processing
     │
     ▼
Report Document
     │
     ▼
Stream
     │
     ▼
Chunk Processing
     │
     ▼
Parquet
     │
     ▼
S3 / GCS
```

という複数の実行段階が存在します。

Amazon側のReport生成完了やS3/GCSへのArtifact確定を、DBトランザクションのCommitとして扱うことはしません。

それぞれの処理段階について、成功・失敗・Retry・Restartの意味を明確にします。

---

## 8. Resource管理の汎用化

Surfinでは、データベース以外のリソースも実行エンジンから適切に管理できるよう、Resourceのライフサイクルを汎用化します。

対象となるリソースには、例えば以下があります。

* Database
* Local File
* S3 Object
* GCS Object
* HTTP Stream
* その他の外部リソース

ただし、これらをすべてTransactionとして扱うのではなく、それぞれの性質に応じたライフサイクルを持たせます。

**注意**: ResourceProviderは、リソースの取得・構築を抽象化するものであり、TransactionのCommit/RollbackやArtifactのFinalizationまでを共通化するものではありません。

---

## 9. JobFactoryとResourceProvider

JobFactoryでは、Job実行に必要なResourceを管理できるようにします。

ResourceProviderは、データベースに限定せず、各種リソースを提供できる抽象化とします。

```text
JobFactory
    │
    ├── Database Resource
    ├── Storage Resource
    ├── HTTP Resource
    └── Other Resources
```

これにより、Application側で利用するInfrastructureを差し替えやすくします。

例えば、

```text
Storage
  ├── Local
  ├── S3
  └── GCS
```

のような差し替えが可能になります。

---

## 10. Execution Semanticsとの関係

Transaction、Resource Lifecycle、Artifact Finalizationは、SurfinのExecution Semanticsの一部として整合性を持って管理します。

特に以下の状態を明確にします。

* TransactionがCommitされたか
* TransactionがRollbackされたか
* Resourceが正常にCloseされたか
* Artifactが正常にFinalizeされたか
* Checkpointが保存されたか
* Job/Step/Chunkが成功したか
* Restart時にどこから再開するか

これらは必ずしも同一の状態になるとは限りません。

例えば、DB TransactionがCommitされた後にArtifact Finalizationが失敗する可能性があります。

その場合、DBのRollbackによってArtifactの状態を戻すことはできません。

したがって、Surfinではこのような異なるリソース間の整合性を「分散トランザクション」で解決するのではなく、Execution Semantics、Checkpoint、Retry、Restart、およびArtifact Lifecycleによって明示的に扱います。

---

## 11. 設計上の原則

Surfinのトランザクション管理では、以下を原則とします。

1. **DB TransactionとResource Lifecycleを分離する**
2. **Contextは実行コンテキストの伝播に利用し、Transactionのライフサイクル自体はExecution Engineが管理する**
3. **S3/GCS/FileなどをDB Transactionとして扱わない**
4. **ResourceのClose/FlushとTransactionのCommitを同一視しない**
5. **生成途中のArtifactと最終Artifactを区別する**
6. **異なるリソース間のAtomicityを暗黙に仮定しない**
7. **Transaction、Checkpoint、Artifact Finalizationの関係をExecution Semanticsとして明示する**
8. **外部サービスとの非同期処理も独立した実行状態として扱う**

この設計により、Surfinはデータベース中心のバッチだけでなく、

```text
External API
    ↓
Stream
    ↓
Chunk Processing
    ↓
File / Parquet
    ↓
S3 / GCS
```

のようなCloud Nativeなデータ処理も、DBトランザクションとは独立した形で扱うことができます。
