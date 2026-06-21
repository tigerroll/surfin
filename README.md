<p align="center">
  <img src="docs/images/surfin-logo.png" alt="Surfin Logo" width="300"/>
</p>

# 🌊 Surfin - Batch framework

[![GoDoc](https://pkg.go.dev/badge/github.com/tigerroll/surfin.svg)](https://pkg.go.dev/github.com/tigerroll/surfin) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/tigerroll/surfin/blob/main/LICENSE) [![Go Report Card](https://goreportcard.com/badge/github.com/tigerroll/surfin)](https://goreportcard.com/report/github.com/tigerroll/surfin)

English | [日本語](./README.ja.md)

A Cloud Native Batch framework for Go, inspired by JSR-352.

Surfin provides the reusable infrastructure that large-scale batch processing requires: logging/tracing, transaction management, job execution statistics, restart, skip, and resource management. It also offers higher-level technical services — optimization and partitioning — that enable extremely large, high-performance batch jobs. From simple jobs to large, complex ones, Surfin lets you process massive datasets with high scalability.

## Restartable Batch Processing Framework for Go

If a job fails partway through, you don't start over. Surfin brings the knowledge encoded in JSR-352 to Go, so batch systems stay maintainable over the long run.

### 😱 Have you ever faced these challenges?

**If any of these sound familiar, Surfin is for you.**

* A batch job failed midway, and nobody knew how far it had gotten.
* You reran it from the start. The next morning, the data was duplicated.
* Someone built a table to track restart flags. Whoever understood the schema has since left the company.
* The logic for "has this been processed" is slightly different in every job.
* You were told "just make it idempotent" — the implementation cost turned out to be far higher than expected.
* Every incident turns into a discussion about where it's safe to resume from.
* The person who owned the batch system moved on, and the design intent went with them.

### 🎯 Use Cases

**Surfin provides the operational primitives that large-scale data processing needs out of the box** — API integration, ETL, data sync, report generation, data lake ingestion, and more.

* **SaaS data integration**: `External API → CSV Stream → Transform → Database → Parquet → Data Lake`
* **ETL / data platform**: `API → Transform → Iceberg → Analytics`
* **Enterprise system integration**: `ERP → Batch → Data Warehouse`
* **Report generation**: `Database → Aggregation → CSV / PDF`
* **IoT / factory data**: `Sensor Data → Batch Processing → Parquet → Data Lake`

Focus on **what to process**. Surfin handles **how to process it safely**.

## 🐹 Motivation: Why Surfin?

### The dilemma of batch systems written in Go

Go favors simplicity, and that simplicity pays off in batch processing.

- ⚡ Native concurrency (goroutines) for efficient large-scale data processing
- 📦 Single binary, simple to deploy
- 🚀 Fast startup, a good fit for Kubernetes Jobs

Go has real advantages for batch workloads on modern infrastructure: a small memory footprint, millisecond startup, and the portability of a single binary — all of which line up well with short-lived, cloud-native environments like AWS ECS Tasks, GCP CloudRun, Kubernetes Jobs, or FaaS. You can also reuse the same Go code (DB schemas, domain logic) from your web API as a shared asset.

But that simplicity comes with a catch.

Writing the happy path with a plain `for rows.Next()` loop is easy. So is making it fast with goroutines:

```go
// concurrent processing with errgroup
eg, ctx := errgroup.WithContext(ctx)

for _, item := range items {
    item := item
    eg.Go(func() error {
        // a single error cancels the context,
        // stopping every other in-flight item too
        return process(ctx, item)
    })
}

if err := eg.Wait(); err != nil {
    return err
}
```

The hard part comes after that.

The moment you try to build a production-grade batch architecture, you're on your own with a long list of distributed-systems problems:

* **Recovery:** If the process above crashes at item 50,000 of 1,000,000, how do you retry safely — without duplicating or double-processing data?
* **Error isolation:** If one bad record should be skipped while the other 999,999 keep processing, how do you enforce a thread-safe error budget (e.g., halt if failures exceed 1%)?
* **Imperfect distributed locks:** How do you prevent double-starts across processes while still handling the lock that never gets released after a container is killed (`SIGKILL`)?
* **3 a.m. decisions (observability):** When an alert fires at 3 a.m., can the on-call engineer tell from the logs alone what's affected and what to do next?

Surfin brings the knowledge built up in enterprise systems over the years into modern Go development.

Go is a relatively young language. The problems batch processing has to deal with are not: restartability, recovery, checkpointing, job management, operational visibility. These have been worked on for decades.

Cloud platforms came and went. Containers became the norm. AI agents started doing the work. None of that changed what large-scale data processing actually requires.

**We don't discard that knowledge. We build on it.**

## 🚀 Getting Started with Surfin

Installation is straightforward.

```bash
go get github.com/tigerroll/surfin
```

👉 Start with the **[Hello, World! tutorial](./docs/tutorial/hello-world.md)**.

A simple job needs only minimal YAML.

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

Business logic is implemented in Go.

```go
func (p *ReportProcessor) Process(
    ctx context.Context,
    item Report,
) (ReportRecord, error) {
    return transform(item), nil
}
```

Flow and business logic are separate. Changing the flow doesn't require touching Go code.

#### A more realistic JSL (Job Specification Language) example

Transitions between steps, item-level retry/skip policies, and chunk size are all declared in YAML.

```yaml
id: myJob
name: Sample Job

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

The job's structure (Job → Step → Chunk) and its fault-tolerance settings (retry/skip) are expressed without writing any code.

## 📍 Key Problems Solved

**You don't know how far it got**

`JobRepository` and `ExecutionContext` persist progress at the chunk level.

```
Chunk #1 ✓
Chunk #2 ✓
Chunk #3 ✓
Chunk #4 ✗  ← resumes from here on rerun
```

**Double execution is a risk**

If the same job is started twice, one of the two runs is rejected automatically.

**You don't want to track resume points manually**

Completed steps are skipped automatically. Only the failed step is rerun.

**You don't want to write retry logic every time**

Declare it as a policy.

```yaml
faultTolerance:
  retry:
    maxAttempts: 3
  skip:
    limit: 100
```

## ♻️ Mechanism of Resume

Surfin persists `ExecutionContext` to the database on every chunk commit. On rerun, it restores that position and resumes from the failure point.

The only thing you need to implement is saving and restoring the current position in your Reader.

```go
// Reader saves its current position to ExecutionContext
func (r *MyReader) Update(ctx context.Context, ec *model.ExecutionContext) error {
    ec.PutInt("read.offset", r.currentOffset)
    return nil
}

// Open restores the position on rerun
func (r *MyReader) Open(ctx context.Context, ec *model.ExecutionContext) error {
    if offset, ok := ec.GetInt("read.offset"); ok {
        r.currentOffset = offset
    }
    return nil
}
```

The framework handles the rest: detecting the failed `JobExecution`, restoring context, and skipping completed steps.

## ⚖️ Comparison with Existing Solutions

You can build all of this yourself. Many teams do.

But the moment restartability, fault tolerance, and safe concurrency become requirements, the cost of a custom implementation climbs fast.

**Before it becomes a job that works but nobody wants to touch.**

Continuing to build these requirements from scratch drives up long-term maintenance cost. JSR-352 is the standard for this in the Java ecosystem; Surfin aims to be the equivalent for Go.

| Feature                | Custom (Go) | JSR-352 (Java)    | Surfin (Go)   |
| ---------------------- | ----------- | ----------------- | ------------- |
| Chunk-based processing | custom      | ✅ built-in       | ✅ built-in   |
| Restartability         | custom      | ✅ built-in       | ✅ built-in   |
| Fault tolerance        | custom      | ✅ built-in       | ✅ built-in   |
| Declarative I/O        | custom      | ✅ built-in       | ✅ built-in   |
| Transaction management | custom      | ✅ built-in       | ✅ built-in   |
| Observability          | custom      | ✅ built-in       | ✅ built-in   |
| Parallel execution     | custom      | ✅ built-in       | ✅ built-in   |
| Job control            | custom      | ✅ built-in       | ✅ built-in   |
| Definition method      | code        | XML/Java Config   | ✅ YAML (JSL) |

## 🏗️ Architecture

Surfin Batch Frameworkは、責務を明確に分離したレイヤードアーキテクチャを採用しています。

詳細なアーキテクチャ図や層構造については、[2. フレームワークの層構造と実行フロー](./docs/architecture/02_architecture.md) を参照してください。

## 🛠️ Key Features

* **📦 Chunk-based processing**: Progress tracking via chunked execution and checkpoints.
* **♻️ Restartability**: Resume precisely from the point of failure; completed steps are skipped automatically.
* **🛡️ Fault tolerance**: Retry, skip, and backoff, declared as policy.
* **📋 Declarative I/O & pipeline**: Job definitions in YAML (JSL), with Reader/Writer cleanly separated.
* **🔄 Transaction management**: Robust transaction handling, including `REQUIRED` and `REQUIRES_NEW` propagation.
* **✨ Observability**: OpenTelemetry and Prometheus integrated at the core.
* **📈 Parallel execution**: Split, Decision, and Partition for parallelism and scaling.
* **🔒 Job control**: Optimistic locking prevents double-starts; full job lifecycle (start/stop) management.

## 📚 Documentation & Usage

* [Introduction & core concepts](./docs/guide/01_introduction.md)
* [Setup & JSL definition](./docs/guide/02_setup_and_jsl.md)
* [Step types & components](./docs/guide/03_step_types_and_components.md)
* [Fault Tolerance & transaction management](./docs/guide/04_fault_tolerance.md)
* [Scaling & parallel processing](./docs/guide/05_scaling_and_parallelism.md)
* **Architecture & Design**
    * [Project Overview](./docs/architecture/00_project_overview.md)
    * [Vision & Design Principles](./docs/architecture/01_vision_and_principles.md)
    * [Architecture Overview](./docs/architecture/02_architecture.md)
    * [Fault Tolerance & TX](./docs/architecture/03_fault_tolerance_and_tx.md)
* [Implementation roadmap](./docs/strategy/adapter_and_component_roadmap.md)

## 🆘 Support

Questions, bug reports, and feature requests go through GitHub Issues.

* **GitHub Issues**: [Report bugs / request features](https://github.com/tigerroll/surfin/issues)

## 📄 License

MIT
