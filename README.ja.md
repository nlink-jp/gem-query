# gem-query

Vertex AI Geminiを使ったDuckDB/SQLiteの自然言語データ分析CLI。

自然言語で質問を入力すると、SQLを自動生成し、ドライランで検証した上で対話的に実行する。インタラクティブなDBシェルとしても、パイプフレンドリーなワンショットCLIとしても使用可能。

## 前提条件

- **Google Cloudプロジェクト** — Vertex AI APIが有効であること
- **Application Default Credentials** — `gcloud auth application-default login` を実行
- **DuckDB** データベースファイル（`.duckdb` または `.sqlite`）

## インストール

```bash
git clone https://github.com/nlink-jp/gem-query.git
cd gem-query
make build
# バイナリ: dist/gem-query
```

> **注意:** DuckDBはCGOが必要です。`make build` は自動的に `CGO_ENABLED=1` を設定します。

## 設定

設定は以下の順序で読み込まれる（後のものが優先）：

1. **デフォルト値** — 組み込み値
2. **TOMLファイル** — `~/.config/gem-query/config.toml`（または `-c` フラグで指定）
3. **環境変数** — `GEMQUERY_*`（ツール固有） > `GOOGLE_CLOUD_*`（汎用）
4. **CLIフラグ** — 最優先

### 設定ファイル

サンプルをコピーして編集：

```bash
mkdir -p ~/.config/gem-query
cp config.example.toml ~/.config/gem-query/config.toml
```

```toml
[gcp]
project  = "your-project-id"
location = "us-central1"

[model]
name = "gemini-2.5-flash"

[tools]
# jviz_path = "/usr/local/bin/jviz"
```

### 環境変数

| 変数 | 必須 | デフォルト | 説明 |
|------|------|-----------|------|
| `GEMQUERY_PROJECT` | はい | — | GCPプロジェクトID |
| `GEMQUERY_LOCATION` | いいえ | `us-central1` | Vertex AIリージョン |
| `GEMQUERY_MODEL` | いいえ | `gemini-2.5-flash` | Geminiモデル名 |
| `GEMQUERY_JVIZ_PATH` | いいえ | — | jvizバイナリのパス |
| `GOOGLE_CLOUD_PROJECT` | — | — | `GEMQUERY_PROJECT`のフォールバック |
| `GOOGLE_CLOUD_LOCATION` | — | — | `GEMQUERY_LOCATION`のフォールバック |

## 使い方

### インタラクティブモード

```bash
gem-query ./data.duckdb
```

```
gem-query> 顧客別の売上合計を上位10件で教えて

[SQL]
  SELECT customer_name, SUM(amount) AS total
  FROM sales GROUP BY customer_name
  ORDER BY total DESC LIMIT 10;

Execute? [Y/n/e(dit)]: y

+---------------+--------+
| customer_name | total  |
+---------------+--------+
| Acme Corp     | 6500   |
| ...           | ...    |
+---------------+--------+
10 rows

gem-query> /jviz
jviz started. Query results will auto-update in the browser.

gem-query> 月別に分解して
  → SQL生成、実行、テーブル表示、jviz自動更新

gem-query> /export json result.json
gem-query> /sql --clipboard
gem-query> /quit
```

### シェルコマンド

| コマンド | 説明 |
|---------|------|
| `/sql` | 直前のSQLを表示 |
| `/sql --clipboard` | 直前のSQLをクリップボードにコピー |
| `/sql <file>` | 直前のSQLをファイルに保存 |
| `/export <json\|csv> <file>` | 結果をファイルにエクスポート |
| `/export <json\|csv> --clipboard` | 結果をクリップボードにエクスポート |
| `/summarize` | 結果をLLMで要約 |
| `/jviz` | jvizライブモード開始（クエリ結果で自動更新） |
| `/jviz --port <port>` | ポート指定でjviz起動 |
| `/jviz off` | jviz停止 |
| `/format <table\|json\|csv>` | 表示形式の切り替え |
| `/help` | ヘルプ表示 |
| `/quit` | 終了 |

### ワンショットモード

```bash
# テーブル出力（デフォルト）
gem-query ./data.duckdb "顧客別売上上位10件"

# JSON出力（パイプ向け）
gem-query ./data.duckdb "月別売上" --format json

# CSV出力
gem-query ./data.duckdb "地域別売上" --format csv

# LLM要約付き
gem-query ./data.duckdb "カテゴリ別集計" --summarize

# jvizにパイプして可視化
gem-query ./data.duckdb "月別売上" --format json | jviz
```

### フラグ

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `-c, --config` | `~/.config/gem-query/config.toml` | 設定ファイルパス |
| `-m, --model` | (設定値) | モデル名の上書き |
| `--format` | `table` | 出力形式: `table`, `json`, `csv` |
| `--jviz` | (設定値) | jvizバイナリのパス |
| `--summarize` | `false` | LLMで結果を要約 |
| `--debug` | `false` | デバッグ出力を有効化 |

## 仕組み

```
質問 → LLMがSQL生成（スキーマコンテキスト付き）
        → ドライラン検証（EXPLAIN）
          → 構文エラー時は自動修正ループ
            → ユーザーがSQL承認
              → 実行 → 結果表示
                → コンテキストを次の質問に引き継ぎ
```

1. **スキーマ認識** — 起動時にDuckDBから全テーブル・カラムのメタデータを取得
2. **SQL生成** — 自然言語 + スキーマをGeminiに送信し、SQLを受け取る
3. **ドライラン検証** — `EXPLAIN`で実行前に構文エラーを検出
4. **自動修正ループ** — ドライランが失敗した場合、エラーをGeminiにフィードバックして修正（最大3回）
5. **ユーザー確認** — 提案されたSQLを常に表示し、承認を求める
6. **コンテキスト継続** — 前回のSQL・結果を後続の質問に引き継ぐ
7. **セキュリティ** — ユーザー入力はノンスタグXML（nlk/guard）でラッピングしてプロンプトインジェクションを防止。SELECTクエリのみ生成

## ビルド

```bash
make build       # ビルド → dist/gem-query
make build-all   # クロスコンパイル（darwin は arm64 のみ・Intel 非対応、Linux/WindowsはPodman/Docker必要）
make test        # 全テスト実行
make check       # vet → test → build
make clean       # dist/を削除
```

## ドキュメント

- [RFP](docs/ja/gem-query-rfp.ja.md) — 要件定義書

## ライセンス

[LICENSE](LICENSE)を参照。
