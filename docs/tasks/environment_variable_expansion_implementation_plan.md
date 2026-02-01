# 環境変数プレースホルダー展開機能の段階的実装計画

本ドキュメントは、YAML設定ファイル（`application.yaml` および JSLファイル）における環境変数プレースホルダー展開機能の具体的な実装ステップを、既存のシステム動作を維持しながら進めるための計画を記述します。

## 1. 前提

*   `docs/strategy/environment_variable_expansion_design.md` に記述された設計に合意していること。
*   Go言語の標準ライブラリ `os.ExpandEnv` を使用して環境変数を展開すること。
*   YAMLのアンマーシャルには `gopkg.in/yaml.v3` を使用すること。

## 1.5. 進捗管理

以下のテーブルで各実装ステップの進捗を管理します。

| ステップ番号 | ステップ名                                   | 状態   | 備考 |
| :----------- | :------------------------------------------- | :----- | :--- |
| 1            | `EnvironmentExpander` インターフェースと実装の作成 | 完了   |      |
| 2            | `application.yaml` のロード処理への組み込み  | 完了   |      |
| 3            | JSL (Job Specification Language) のロード処理への組み込み | 完了   |      |

## 2. 実装ステップ

### ステップ 1: `EnvironmentExpander` インターフェースと実装の作成

このステップでは、環境変数展開のロジックをカプセル化するインターフェースと、そのデフォルト実装を新規に作成します。既存のコードには一切変更を加えないため、このステップ完了後もシステムの動作は完全に維持されます。

**目的:** 環境変数展開の抽象化レイヤーを導入する。

**変更ファイル:**
*   `pkg/batch/core/config/environment_expander.go` (新規作成)

**変更内容:**

`pkg/batch/core/config/environment_expander.go` を以下の内容で作成します。

```go
// pkg/batch/core/config/environment_expander.go

package config

import (
	"os"
)

// EnvironmentExpander は、入力バイト列内の環境変数プレースホルダーを展開する機能を提供します。
type EnvironmentExpander interface {
	// Expand は、与えられたバイト列内の環境変数プレースホルダーを展開し、
	// 展開されたバイト列を返します。
	Expand(input []byte) ([]byte, error)
}

// OsEnvironmentExpander は、os.ExpandEnv を使用して環境変数を展開する EnvironmentExpander の実装です。
type OsEnvironmentExpander struct{}

// NewOsEnvironmentExpander は OsEnvironmentExpander の新しいインスタンスを返します。
func NewOsEnvironmentExpander() *OsEnvironmentExpander {
	return &OsEnvironmentExpander{}
}

// Expand は、os.ExpandEnv を使用して入力バイト列内の環境変数を展開します。
// os.ExpandEnv はエラーを返さないため、ここでは常にnilを返します。
func (e *OsEnvironmentExpander) Expand(input []byte) ([]byte, error) {
	expandedString := os.ExpandEnv(string(input))
	return []byte(expandedString), nil
}
```

**検証:**
*   このステップでは、既存のコードベースに影響を与えないため、特別な検証は不要です。ビルドが成功することを確認してください。

### ステップ 2: `application.yaml` のロード処理への組み込み

このステップでは、アプリケーションのメイン設定ファイルである `application.yaml` のロード処理に、ステップ1で作成した `EnvironmentExpander` を組み込みます。

**目的:** `application.yaml` 内の環境変数をアプリケーション起動時に展開できるようにする。

**変更ファイル:**
*   `main.go` またはアプリケーションの初期化を担当する設定ロード関数が含まれるファイル。
    *   **注:** 正確なファイルパスは、`application.yaml` を読み込み、`pkg/batch/core/config.Config` にアンマーシャルしている箇所によって異なります。

**変更内容 (例: `main.go` 内の `LoadApplicationConfig` 関数を想定):**

```go
// 変更前 (概念的なコード)
// func LoadApplicationConfig(configPath string) (*config.Config, error) {
//     data, err := ioutil.ReadFile(configPath)
//     // ...
//     cfg := config.NewConfig()
//     err = yaml.Unmarshal(data, cfg)
//     // ...
// }

// 変更後 (概念的なコード)
import (
	"io/ioutil"
	"gopkg.in/yaml.v3"
	"fmt"
	"github.com/tigerroll/surfin/pkg/batch/core/config" // configパッケージをインポート
)

func LoadApplicationConfig(configPath string) (*config.Config, error) {
	// 1. YAMLコンテンツの読み込み
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// 2. 環境変数の展開
	// config.NewOsEnvironmentExpander() を使用してインスタンスを作成
	expander := config.NewOsEnvironmentExpander()
	expandedData, err := expander.Expand(data)
	if err != nil {
		// os.ExpandEnvはエラーを返さないため、このerrは常にnilですが、将来的な拡張性を考慮して残します。
		return nil, fmt.Errorf("failed to expand environment variables in config: %w", err)
	}

	// 3. Go構造体へのアンマーシャル
	cfg := config.NewConfig() // デフォルト値で初期化
	err = yaml.Unmarshal(expandedData, cfg) // 展開されたデータを使用
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal expanded config: %w", err)
	}

	// (オプション) アンマーシャル後の設定値の検証ロジックがあれば、ここに呼び出しを追加

	return cfg, nil
}
```

**検証:**
*   アプリケーションをビルドし、起動します。
*   `application.yaml` 内に、例えば `api_key: ${MY_API_KEY}` のように環境変数プレースホルダーを含む設定を追加します。
*   `MY_API_KEY` 環境変数を設定してアプリケーションを起動し、設定が正しくロードされ、`MY_API_KEY` の値が反映されていることを確認します（例: ログ出力やデバッガで確認）。
*   `MY_API_KEY` 環境変数を設定せずにアプリケーションを起動し、空文字列として扱われること、または適切なエラーハンドリングが行われることを確認します。
*   既存の機能が引き続き正常に動作することを確認します。

### ステップ 3: JSL (Job Specification Language) のロード処理への組み込み

このステップでは、JSLファイル（`job.yaml` など）のロード処理に、ステップ1で作成した `EnvironmentExpander` を組み込みます。

**目的:** JSLファイル内の環境変数をジョブ実行時に展開できるようにする。

**変更ファイル:**
*   JSLファイルを読み込み、`pkg/batch/core/config/jsl.Job` にアンマーシャルしている箇所。
    *   **注:** 正確なファイルパスと関数名は、JSLのロードロジックの実装によって異なります。例えば、`pkg/batch/engine/job/loader.go` の `LoadJobDefinition` 関数などが考えられます。

**変更内容 (例: `JobLoader.LoadJobDefinition` 関数を想定):**

```go
// 変更前 (概念的なコード)
// func (jl *JobLoader) LoadJobDefinition(jslPath string) (*jsl.Job, error) {
//     data, err := ioutil.ReadFile(jslPath)
//     // ...
//     job := &jsl.Job{}
//     err = yaml.Unmarshal(data, job)
//     // ...
// }

// 変更後 (概念的なコード)
import (
	"io/ioutil"
	"gopkg.in/yaml.v3"
	"fmt"
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/core/config" // configパッケージをインポート
)

// JobLoader に EnvironmentExpander を依存性注入することを推奨
type JobLoader struct {
	expander config.EnvironmentExpander // EnvironmentExpanderを依存性注入
	// ... 他のフィールド
}

// NewJobLoader は JobLoader の新しいインスタンスを生成します。
// 依存性注入の例
func NewJobLoader(expander config.EnvironmentExpander /*, ...他の依存性 */) *JobLoader {
	return &JobLoader{expander: expander /*, ... */}
}

func (jl *JobLoader) LoadJobDefinition(jslPath string) (*jsl.Job, error) {
	// 1. JSLコンテンツの読み込み
	data, err := ioutil.ReadFile(jslPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSL file %s: %w", jslPath, err)
	}

	// 2. 環境変数の展開
	expandedData, err := jl.expander.Expand(data) // 注入されたexpanderを使用
	if err != nil {
		return nil, fmt.Errorf("failed to expand environment variables in JSL: %w", err)
	}

	// 3. Go構造体へのアンマーシャル
	job := &jsl.Job{}
	err = yaml.Unmarshal(expandedData, job) // 展開されたデータを使用
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal expanded JSL: %w", err)
	}

	// (オプション) アンマーシャル後のJSL定義の検証ロジックがあれば、ここに呼び出しを追加

	return job, nil
}
```

**検証:**
*   JSLファイル内に、例えば `description: "Job for ${ENV_NAME} environment"` のように環境変数プレースホルダーを含む定義を追加します。
*   `ENV_NAME` 環境変数を設定してジョブを実行し、ジョブ定義が正しくロードされ、`ENV_NAME` の値が反映されていることを確認します（例: ジョブのログやメタデータで確認）。
*   `ENV_NAME` 環境変数を設定せずにジョブを実行し、空文字列として扱われること、または適切なエラーハンドリングが行われることを確認します。
*   既存のジョブが引き続き正常に実行されることを確認します。
