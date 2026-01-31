# YAML設定ファイルにおける環境変数プレースホルダー展開機能の設計

## 1. 目的

本ドキュメントは、Surfin Batch Frameworkにおいて、`application.yaml` および Job Specification Language (JSL) ファイル (`job.yaml` など) 内に記述された `${VALUE}` 形式の環境変数プレースホルダーを、アプリケーション起動時またはジョブ定義ロード時に実際の環境変数の値に置き換える機能の設計を記述します。これにより、設定の柔軟性と環境ごとの設定管理の容易性を向上させます。

## 2. 現状の課題

現在のシステムでは、YAML設定ファイル内に記述された `${VALUE}` 形式のプレースホルダーが環境変数の値に自動的に置き換えられる機能が実装されていません。このため、環境ごとに異なる設定（例: データベース接続情報、APIキーなど）を管理する際に、YAMLファイルを直接編集するか、ビルド時に値を埋め込むなどの手間が発生しています。

## 3. 設計の概要

YAML設定ファイル内の環境変数プレースホルダーを展開する機能は、以下のステップで実現します。

1.  **YAMLコンテンツの読み込み**: `application.yaml` や JSLファイルの内容を、ファイルシステムから生のバイト列 (`[]byte`) として読み込みます。
2.  **環境変数の展開**: 読み込んだバイト列を文字列に変換し、Go標準ライブラリの `os.ExpandEnv()` 関数を使用して、文字列内の `${VAR}` または `$VAR` 形式のプレースホルダーを対応する環境変数の値に置き換えます。
3.  **Go構造体へのアンマーシャル**: 環境変数が展開された後のYAMLコンテンツ（バイト列に戻したもの）を、`gopkg.in/yaml.v3` などのYAMLパーサーライブラリを使用して、対応するGoの構造体（例: `pkg/batch/core/config.Config` や `pkg/batch/core/config/jsl.Job`）にアンマーシャルします。

このアプローチにより、YAMLのパース処理自体に変更を加えることなく、パースの「前段階」で環境変数の置き換えを行います。

## 4. 抽象化の提案

環境変数展開のロジックをより抽象化し、将来的な拡張性、保守性、テスト容易性を高めるため、以下のインターフェースと実装を導入します。

### 4.1. EnvironmentExpander インターフェース

環境変数を展開する機能を表すインターフェースを定義します。

```go
// EnvironmentExpander は、入力バイト列内の環境変数プレースホルダーを展開する機能を提供します。
type EnvironmentExpander interface {
	// Expand は、与えられたバイト列内の環境変数プレースホルダーを展開し、
	// 展開されたバイト列を返します。
	Expand(input []byte) ([]byte, error)
}
```

### 4.2. OsEnvironmentExpander 実装

`os.ExpandEnv` を利用した `EnvironmentExpander` のデフォルト実装を提供します。

```go
// OsEnvironmentExpander は、os.ExpandEnv を使用して環境変数を展開する EnvironmentExpander の実装です。
type OsEnvironmentExpander struct{}

// NewOsEnvironmentExpander は OsEnvironmentExpander の新しいインスタンスを返します。
func NewOsEnvironmentExpander() *OsEnvironmentExpander {
	return &OsEnvironmentExpander{}
}

// Expand は、os.ExpandEnv を使用して入力バイト列内の環境変数を展開します。
func (e *OsEnvironmentExpander) Expand(input []byte) ([]byte, error) {
	// os.ExpandEnv は文字列を扱うため、バイト列と文字列の変換が必要です。
	expandedString := os.ExpandEnv(string(input))
	return []byte(expandedString), nil
}
```

### 4.3. 利用箇所での組み込み

`application.yaml` や JSLファイルを読み込む既存の処理において、`yaml.Unmarshal()` を呼び出す直前に、`EnvironmentExpander` のインスタンスを使用してファイルの内容を展開するように変更します。

## 5. 実装場所

上記の `EnvironmentExpander` インターフェースおよび `OsEnvironmentExpander` 実装は、`pkg/batch/core/config/` パッケージ内に新しいファイルとして作成します。

推奨ファイル名: `pkg/batch/core/config/environment_expander.go`

## 6. 利点

この設計アプローチには以下の利点があります。

*   **抽象化とカプセル化**: 環境変数展開の具体的なロジックがインターフェースの背後に隠蔽され、コードの可読性と保守性が向上します。
*   **柔軟性**: 将来的に環境変数の解決方法を変更する必要が生じた場合（例: カスタムのプレースホルダー形式、設定管理サービスからの値取得など）でも、`EnvironmentExpander` インターフェースの新しい実装を作成するだけで済み、既存のロードロジックに大きな変更を加える必要がありません。
*   **テスト容易性**: `EnvironmentExpander` がインターフェースであるため、ユニットテストにおいてモックやスタブを容易に作成し、環境変数展開の挙動をシミュレートできます。
*   **単一責任の原則**: ファイルの読み込み、環境変数展開、YAMLのアンマーシャルという異なる責任が、それぞれ独立したコンポーネントやステップに分離されます。

## 7. 考慮事項とリスク

環境変数展開機能の実装にあたり、以下の点に留意する必要があります。

*   **環境変数が未設定の場合**: YAML内で指定された環境変数が実行環境で設定されていない場合、`os.ExpandEnv` はそのプレースホルダーを空文字列に置き換えます。これにより、後続の `yaml.Unmarshal` が成功しても、設定値が空となり、アプリケーションの動作に予期せぬ影響を与える可能性があります。これは設定ミスとして、アプリケーション側で適切な検証を行う必要があります。
*   **環境変数の値がYAML構文を破壊する可能性**: 環境変数の値にYAMLの特殊文字（例: `:`、`"`、`'`、改行など）が含まれている場合、展開後のYAML文字列が不正な構文となり、`yaml.Unmarshal` がパースエラーを返す可能性があります。
    *   この問題に対する手動での文字列サニタイズは複雑でバグを生みやすいため推奨されません。
    *   `yaml.Unmarshal` が返すエラーを適切にハンドリングし、詳細なログを出力することで、問題の特定を容易にします。
    *   アンマーシャル後のGo構造体に対して、各フィールドの**値の検証 (Validation)** を行うことで、不正な設定値がアプリケーションに渡されることを防ぎます。
    *   YAMLファイル内で環境変数を参照する際は、`key: "${ENV_VAR}"` のように引用符で囲むことで、値に含まれる特殊文字がYAMLパーサーによって正しく解釈されるように運用ルールを設けることが推奨されます。
