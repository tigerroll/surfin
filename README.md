<p align="center">
  <img src="docs/images/surfin-logo.png" alt="Surfin Logo" width="150"/>
</p>
<center>
  [English](README.md) | [日本語](README.ja.md)
</center>

# 🌊 Surfin - Batch framework

[![GoDoc](https://pkg.go.dev/badge/github.com/tigerroll/surfin.svg)](https://pkg.go.dev/github.com/tigerroll/surfin)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/tigerroll/surfin)](https://goreportcard.com/report/github.com/tigerroll/surfin)

**A lightweight batch framework for Go, inspired by JSR-352.**

**Surfin - Batch framework** is being developed with robustness, scalability, and operational ease as top priorities. With declarative job definitions (JSL) and a clean architecture, it efficiently and reliably executes complex data processing tasks.

---

## 🌟 Why Choose Surfin?

*  **🚀 Go Performance:** Leverages Go's native concurrency and performance to the fullest.
*  **🏗️ Convention over Configuration (CoC):** Eliminates boilerplate code through Convention over Configuration (CoC), ensuring robustness with minimal implementation.
*  **✨ Observable:** Integrates Prometheus/OpenTelemetry at its core, enabling distributed tracing and metrics collection simply by following conventions.
*  **🛠️ Flexible:** Supports dynamic DB routing and Uber Fx (DI) for flexible adaptation to complex infrastructures and custom requirements.
*  **🔒 Robust:** Provides fault tolerance features through persistent metadata, optimistic locking, and fine-grained error handling (retry/skip).
*  **🎯 JSR-352 Compliant:** Implements a batch processing model inspired by the industry standard (JSR-352).
*  **📈 Scalable:** Supports large-scale datasets through distributed execution via remote worker integration and abstraction of Partitioning.

---

## 🛠️  Key Features

### ⚙️ Core Features and Flow Control
*   **Declarative Job Definition (JSL)**: Define jobs, steps, components, and transitions declaratively using YAML-based JSL.
*   **Chunk/Tasklet Model**: Supports chunk-oriented processing with `ItemReader`, `ItemProcessor`, `ItemWriter`, and `Tasklet` for single-task execution.
*   **Advanced Flow Control**: Build complex job flows with conditional branching (`Decision`), parallel execution (`Split`), and flexible transition rules (`Transition`).
*   **Listener Model**: Custom logic can be injected into Job, Step, Chunk, and Item lifecycle events.

### 🛡️ Robustness and Metadata Management
*   **Restartability**: Failed jobs can be accurately restarted from their interruption point using `JobRepository` and `ExecutionContext`.
*   **Fault Tolerance**: Supports item-level **retry** and **skip** policies. Automatically applies chunk-splitting skip logic for write errors.
*   **Optimistic Locking**: Implements optimistic locking at the metadata repository layer to ensure data consistency in distributed environments.
*   **Sensitive Information Masking**: Automatically masks sensitive information when `JobParameters` are persisted and logged.

### ✨ Operational Ease and Observability
*   **OpenTelemetry/Prometheus Integration:** Integrates distributed tracing (automatic Job/Step Span generation) and metrics collection (MetricRecorder) into the framework's core. Ensures reliable monitoring even for short-lived batches and provides a foundation for integration with Remote Schedulers.
*   **Fine-grained Error Policy:** Supports declarative retry/skip strategies based on error characteristics (Transient, Skippable).

### 🌐 Scalability and Infrastructure
*   **Dynamic DI**: Adopts Uber Fx for dependency injection, enabling dynamic component construction and lifecycle management.
*   **Dynamic Data Source Routing**: Supports dynamic switching between multiple data sources (Postgres, MySQL, etc.) based on execution context.
*   **Distributed Execution Abstraction**: Abstraction of `Partitioner` and `StepExecutor` supports delegation of execution from local to remote workers (e.g., Kubernetes Job).

---

## 🚀 Getting Started

Refer to the following guide to build a minimal application using Surfin.

👉 **[tutorial - "Hello, Wold!"](docs/tutorial/hello-world.md)**

## 📚 Documentation & Usage

For more detailed features, configurations, and building complex job flows, refer to the following documentation.

*   **[Architecture and Design Principles](docs/architecture)**: Refer to detailed information on the framework's architecture, design principles, and project structure.
*   **[Guide to Creating Surfin Batch Applications](docs/guide/usage.md)**: A complete guide to JSL, custom components, listeners, and flow control.
*   **[Implementation Roadmap](docs/strategy/adapter_and_component_roadmap.md)**: The framework's design goals and progress.

---

## 🆘 Support

If you have any questions or encounter issues, please use GitHub Issues.

*   **GitHub Issues**: [Report bugs or request features](https://github.com/tigerroll/surfin/issues)
----
