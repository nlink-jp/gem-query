# RFP: gem-query

> Generated: 2026-04-14
> Status: Draft

## 1. Problem Statement

gem-query は、SQL に不慣れなエンジニアが自然言語で DuckDB / SQLite のデータを対話的に分析するための CLI ツールである。ユーザーの自然言語指示からテーブルスキーマを考慮した SQL を LLM（Vertex AI Gemini）が動的に生成し、ドライランで構文チェック・自動修正した上で実行する。結果はデータとしてそのまま表示し、要約はオプション扱いとする。

**ターゲットユーザー:** SQL をある程度読めるが、複雑なクエリを一から書くのは得意ではないエンジニア。

## 2. Functional Specification

### Commands / API Surface

**起動モード:**

- **インタラクティブモード（デフォルト）:** `gem-query ./data.duckdb` で DB シェルを起動
- **ワンショットモード:** `gem-query ./data.duckdb "売上上位10件"` でパイプ対応の単発実行

**インタラクティブフロー:**

1. ユーザーが自然言語で質問を入力
2. LLM がスキーマ + 会話コンテキストから SQL を生成・提案表示
3. ユーザーが承認 / 拒否 / 編集
4. ドライランで構文チェック → エラー時は LLM による自動修正ループ
5. 実行 → 結果をテーブル表示 + 内部に構造化データ保持
6. 会話コンテキスト継続（直前の SQL・結果を次の質問に引き継ぐ）

**シェルコマンド:**

| コマンド | 説明 |
|---------|------|
| `/export json <file>` | 結果を JSON ファイルに出力 |
| `/export csv <file>` | 結果を CSV ファイルに出力 |
| `/export json --clipboard` | 結果を JSON でクリップボードへ |
| `/export csv --clipboard` | 結果を CSV でクリップボードへ |
| `/sql` | 直前の SQL を表示 |
| `/sql --clipboard` | 直前の SQL をクリップボードへ |
| `/sql <file>` | 直前の SQL をファイルに出力 |
| `/summarize` | 結果を LLM で要約 |
| `/format <table\|json\|csv>` | 表示形式を切り替え |

**ワンショットモード フラグ:**

| フラグ | 説明 |
|-------|------|
| `--format json\|csv\|table` | 出力形式指定（デフォルト: table） |
| `--summarize` | 結果を LLM で要約して出力 |
| `-c, --config <path>` | config.toml のパス指定 |

### Input / Output

- **入力:** DuckDB ファイルパス（起動引数）、自然言語クエリ（対話 or 引数）
- **出力:**
  - インタラクティブ: テーブル表示（デフォルト）、コマンドで JSON / CSV に切替・エクスポート
  - ワンショット: stdout へ table / JSON / CSV 出力（パイプ連結可能）
- **パイプ連携:** `gem-query ./data.duckdb "月別売上" --format json | jviz`

### Configuration

Vertex AI config.toml 統一パターンに準拠。

**config.toml:**

```toml
[gcp]
project  = "your-project-id"
location = "us-central1"

[model]
name = "gemini-2.5-flash"
```

**デフォルトパス:** `~/.config/gem-query/config.toml`

**環境変数（優先度: 環境変数 > TOML > デフォルト）:**

- `GEMQUERY_PROJECT` / `GOOGLE_CLOUD_PROJECT`
- `GEMQUERY_LOCATION` / `GOOGLE_CLOUD_LOCATION`
- `GEMQUERY_MODEL`

### External Dependencies

| 依存 | 用途 |
|------|------|
| Vertex AI Gemini API | 自然言語 → SQL 生成、要約 |
| DuckDB | データベースエンジン（ローカルファイル） |

## 3. Design Decisions

**言語: Go**
- Vertex AI Gemini SDK（google.golang.org/genai）、DuckDB（go-duckdb）、nlk ライブラリとの一貫性。既存 gem-search / gem-image と同じエコシステム。

**TUI: readline 系シンプル REPL**
- util-series に bubbletea の採用実績がなく、メンテナンス負荷を避ける。DB シェル的なプロンプト → 入力 → 結果のループで十分。

**DB: go-duckdb（database/sql 準拠）**
- 標準 database/sql インターフェースにより、将来的な DB バックエンド拡張が容易。

**LLM: google.golang.org/genai（Vertex AI 統一 SDK）**
- deprecated な vertexai/genai ではなく、最新の統一 SDK を使用。

**nlk 統合:**

| パッケージ | 用途 |
|-----------|------|
| `nlk/guard` | プロンプトインジェクション防御（ノンスタグ XML ラッピング） |
| `nlk/backoff` | Gemini API リトライ（指数バックオフ） |
| `nlk/strip` | LLM レスポンスの thinking タグ除去 |
| `nlk/jsonfix` | LLM レスポンスからの JSON 抽出・修復 |
| `nlk/validate` | 出力バリデーション |

**gem-rag との棲み分け:**
- gem-rag = 非構造化ドキュメントの RAG 検索
- gem-query = 構造化データ（DB）の自然言語分析

**スコープ外:**
- DB 書き込み操作（INSERT / UPDATE / DELETE）— SELECT 系のみ
- リモート DB 接続（PostgreSQL 等）— 現時点では不要
- ダッシュボード的な可視化 — jviz に委譲

**将来連携:**
- jviz（JSON 出力パイプラインによる可視化）— Phase 2 以降で検討

## 4. Development Plan

### Phase 1: Core

- プロジェクトスキャフォールド（Cobra CLI, config.toml, Makefile）
- DuckDB 接続 + テーブルスキーマ取得
- Gemini 連携（自然言語 → SQL 生成、nlk/guard 統合）
- SQL ドライラン + 自動修正ループ
- インタラクティブ REPL シェル（質問 → SQL 提案 → 確認 → 実行 → テーブル表示）
- 会話コンテキスト継続（直前の SQL / 結果の引き継ぎ）
- ユニットテスト

**独立レビュー可能**

### Phase 2: Features

- ワンショットモード（パイプ対応、`--format`、`--summarize`）
- `/export`、`/sql`、`/summarize` シェルコマンド群
- クリップボード連携（macOS: pbcopy, Linux: xclip, Windows: clip）
- jviz 連携（JSON 出力パイプ）
- 追加テスト

**独立レビュー可能**

### Phase 3: Release

- README.md / README.ja.md 作成
- CHANGELOG.md / AGENTS.md 作成
- config.example.toml 作成
- E2E テスト（実データでのシミュレーション）
- リリース（タグ、gh release、zip アップロード）
- util-series サブモジュールポインタ更新

## 5. Required API Scopes / Permissions

| 項目 | 値 |
|------|-----|
| 認証方式 | ADC（Application Default Credentials） |
| 取得方法 | `gcloud auth application-default login` |
| IAM ロール | `roles/aiplatform.user` |
| OAuth スコープ | `https://www.googleapis.com/auth/cloud-platform` |

DuckDB はローカルファイルアクセスのみ。追加の API 権限は不要。

## 6. Series Placement

**Series:** util-series

**Reason:** パイプフレンドリーなデータ変換・処理 CLI 群（gem-search, gem-image, gem-rag, jviz 等）と同じシリーズ。gem-query もデータ分析 CLI として自然に収まる。

## 7. External Platform Constraints

| 制約 | 対応 |
|------|------|
| Vertex AI Gemini レート制限（RPM/TPM） | nlk/backoff による指数バックオフリトライ |
| DuckDB は CGO 必須 | クロスコンパイル時に `CGO_ENABLED=1` が必要。ビルド対象プラットフォームが制限される可能性あり |
| クリップボード操作が OS 依存 | macOS (`pbcopy`), Linux (`xclip`/`xsel`), Windows (`clip`) を検出して切り替え |

---

## Discussion Log

### ツール名の選定
gem-search / gem-image / gem-rag との一貫性を重視し、`gem-query` を採用。`ask-db`、`nlq`、`datawise` 等の候補も検討した。

### TUI フレームワークの検討
当初 bubbletea（Charm 系）を候補としたが、調査の結果 util-series に TUI ライブラリの採用実績がなく、DuckDB の CGO 依存と合わせてビルド複雑性が増すため、readline 系シンプル REPL を採用。DB シェル的な UX にはこちらが適切と判断。

### SQL 実行前の確認フロー
ユーザーの要望により、LLM が生成した SQL を自動実行せず、必ずユーザーに提案・確認するフローを採用。SQL のクリップボード / ファイル出力機能も追加し、生成された SQL の再利用性を高めた。

### 結果の要約について
LLM による要約はデフォルトでは行わず、明示的なオプション（`/summarize` コマンド or `--summarize` フラグ）でのみ実行。データ分析の文脈では、生データの確認が優先されるため。

### jviz 連携
可視化は gem-query 自体のスコープ外とし、jviz へのパイプ連携（`--format json | jviz`）で対応する方針。Phase 2 で実装予定。

### DuckDB 選定理由
DuckDB を選択することで SQLite のファイルも読み込み可能。ローカルファイルベースの分析に特化し、リモート DB 接続は現時点でスコープ外とした。
