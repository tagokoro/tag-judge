# Rust Judge 設計方針

## 1. 目的

このプロジェクトの主役は、参加者向け UI ではなく、提出コードを安全かつ非同期に処理するオンラインジャッジ基盤である。

Rust は、最も危険で重要な境界である「提出コードの実行」に集中して使う。

```text
API Server      : 提出を受け付ける
Judge Worker    : 判定ジョブを受け取り、結果を返す
judge-core      : 実際に compile / run / compare する
```

## 2. Rust で作る成果物

### judge-core

判定処理そのものを担う純粋なコア。

```text
入力:
- source code
- test cases
- execution config

出力:
- verdict
- runtime
- case results
```

責務:

- コンパイル
- 実行
- タイムアウト検出
- 出力比較
- verdict 生成

`judge-core` は Queue、DB、Cloudflare などの実行基盤を知らない。これにより、ローカル CLI からも単体テストからも扱いやすくする。

### Cloudflare Consumer Worker / Go judge-runner

インフラとの接続とSandbox内の実行を担う処理。

責務:

- Consumer Worker が Queue から判定ジョブを取得する
- Consumer Worker が R2 から提出コードやテストケースを取得する
- Cloudflare Sandbox 内の Go judge-runner を呼び出す
- 判定結果を返す
- 必要に応じて失敗を再試行可能な形で扱う

MVPでは Consumer Worker がD1へ直接書き込む。runner本体はDBを知らないまま保ち、将来は同じ境界をResult Queue publisherへ差し替えられる構成とする。Cloudflare版ではR2 keyを受け取り、Consumer Workerがobject storageから実体を取得する。

```text
local JudgeJob
- submissionId
- language
- sourceCodeKey
- testCasesKey
- timeLimitMs
```

長期的には判定結果をResult Queueへ発行し、Result ConsumerがDBへ保存する構成を候補とする。

## 3. なぜ分けるのか

```text
judge-core
- Cloudflare を知らない
- Queue を知らない
- DB を知らない
- ただ判定する

Consumer Worker
- Cloudflare の具体bindingを知る
- Queue を知る
- DB を知る
- 仕事を運ぶ

Go judge-runner
- R2 / Queue / D1 を知らない
- Sandbox内で source / testcases / config から判定結果を返す
```

この分離により、次の利点を得る。

- 判定ロジックを単独でテストしやすい
- Queue 実装を変更しても判定ロジックが影響を受けにくい
- ローカル開発では CLI 実行、本番では Worker 経由で利用できる
- Rust を使う理由が明確になる

## 4. MVP の範囲

### 最初に作る

```text
- 1言語のみ対応（まずは Python を想定）
- 標準入力 / 標準出力の通常問題
- run
- 複数テストケース実行
- AC / WA / TLE / RE / IE
- 実行時間計測
- 出力比較
- 構造化された判定結果の返却
```

### 最初は作らない

```text
- 複数言語対応
- C++ などのコンパイルが必要な言語
- 厳密なメモリ制限
- インタラクティブ問題
- Special Judge
- 部分点
- CE の厳密な分類
- 高度な分散最適化
- 管理 UI からの再ジャッジ操作
```

初期段階では、オンラインジャッジの核となる処理を確実に実装し、扱う論点を散らさないことを優先する。

## 5. judge-core の入出力仕様

`judge-core` は、提出コードと複数テストケースを受け取り、ケースごとの判定結果と全体の判定結果を返す。

```text
source code + language + test cases + config
        ↓
    judge-core
        ↓
JudgeResult
```

### JudgeRequest

```text
JudgeRequest
- language
- sourceCode
- timeLimitMs
- testCases[]
```

| フィールド | 役割 |
| --- | --- |
| `language` | 実行する言語。MVP では Python を想定 |
| `sourceCode` | 判定対象の提出コード |
| `timeLimitMs` | 1ケースあたりの実行時間制限 |
| `testCases` | 入力と期待出力の配列 |

本番構成では、提出コードそのものは R2 に保存し、Queue には `sourceCodeKey` のような参照情報を積む。ただし、runner本体は R2 を知らない。Consumer Worker が R2 から取得した実際の `sourceCode` を Sandbox 内の judge-runner に渡す。

API も同じ境界を守る。Submission / CodeRun API は usecase では `SourceCodeStorage` と `ExecutionDispatcher` interface だけに依存し、R2 / Queue の具体 adapter は実行基盤側で組み立てる。Cloudflare版では API Worker が R2 binding と Queue producer binding を持つ。

```text
Queue job
  ↓
Consumer Worker
  ├─ R2 から sourceCode を取得
  ├─ R2 から testCases を取得
  └─ Cloudflare Sandbox 内の Go judge-runner に実データを渡す
```

Queue に積むジョブは Submission 専用の `JudgeJob` ではなく、API Server 側では `ExecutionJob` として扱う。

```text
ExecutionJob
- reference
  - kind: submission | code_run
  - id
- executionMode: problem | stdin
- language
- sourceCodeKey
- problemExecution
  - testCasesKey
  - timeLimitMs
- stdinExecution
  - stdin
  - timeLimitMs
```

`problem` mode は Submission reference と test cases を必須にする。`stdin` mode は CodeRun reference と stdin を必須にする。API Server の constructor validation で、`code_run + problem` や `submission + stdin` のような不正な組み合わせを拒否する。

CodeRun API は `pending` レコードを作成した後、`code-runs/{user_id}/{code_run_id}/main.{ext}` へソースコードを保存し、stdin execution job を dispatch する。実行時間制限は `CODE_RUN_TIME_LIMIT_MS` で設定し、未指定時は 5000ms とする。source storage または dispatch が失敗した場合はエラーを返すが、作成済みの `pending` CodeRun は rollback しない。source key は CodeRun ID から決定的に生成できるため、同じ reference の再送時にも同じartifactを参照できる。

Cloudflare版のConsumer Workerは `reference`, `executionMode`, `submissionId`, `sourceCodeKey`, `testCasesKey`, `timeLimitMs` を読む。既存のRust workerと互換にしやすいよう、Problem execution payloadの基本フィールドは維持する。

Dispatch の責務境界:

- Submission usecase は DB から現在の TestCase 一覧を取得し、不変な TestCase artifact 保存、source code 保存、`pending` Submission 永続化、dispatch の順に実行する
- TestCase artifact は worker が必要な `input` / `expectedOutput` の配列で、`problems/{problem_id}/testcases/{sha256}.json` に保存する
- `testCasesKey` は Submission 受付時点の content-addressed snapshot を指し、後続の TestCase 更新・削除から pending job と再送 job を分離する
- TestCase が 0 件の Problem は Submission を受け付けない
- CodeRun usecase は `pending` CodeRun 永続化、source code 保存、dispatch の順に実行する
- dispatch 失敗時はエラーを返すが、保存済み source code と `pending` record を usecase 内で rollback しない
- queue は少なくとも1回配送を前提にし、同じ `ExecutionJob.reference` の再送を許容する
- runner / result consumer は reference id を冪等キーとして、同じ実行結果の重複反映で状態を壊さない責務を持つ
- 現時点では Submission 作成API自体の冪等キーは提供しないため、HTTP retry は新しい Submission を作る可能性がある

### TestCase

```text
TestCase
- input
- expectedOutput
```

MVP でも単体ケースではなく、複数テストケースを前提とする。オンラインジャッジでは、1つの提出に対して複数ケースを実行し、1ケースでも失敗すれば全体が不正解になるためである。

### JudgeResult

```text
JudgeResult
- verdict
- caseResults[]
- message?
```

| フィールド | 役割 |
| --- | --- |
| `verdict` | 全体の判定結果 |
| `caseResults` | 各テストケースの判定結果 |
| `message` | 人間が読む補足メッセージ |

### CaseResult

```text
CaseResult
- caseIndex
- verdict
- runtimeMs
- stdout
- stderr
- message?
```

| フィールド | 役割 |
| --- | --- |
| `caseIndex` | 何番目のテストケースか |
| `verdict` | そのケースの判定結果 |
| `runtimeMs` | 実行時間 |
| `stdout` | 実際の標準出力 |
| `stderr` | 実際の標準エラー |
| `message` | 補足メッセージ |

### Verdict

```text
AC : Accepted
WA : Wrong Answer
TLE: Time Limit Exceeded
RE : Runtime Error
CE : Compile Error
IE : Internal Error
```

MVP では Python を想定するため、構文エラーなどはまず `RE` として扱う。`CE` は C++ などのコンパイル言語を追加する段階で厳密に扱う。

`IE` は提出コードの失敗ではなく、judge 側の失敗を表す。`RE` と分けることで、ユーザーの提出ミスとシステム障害を混同しない。

### 全体 verdict の決め方

複数ケースの結果から、全体の `verdict` を決める。

```text
IE があれば IE
CE があれば CE
TLE があれば TLE
RE があれば RE
WA があれば WA
すべて AC なら AC
```

MVP では、最初の非 AC ケースで実行を止める方針とする。これにより、実装と実行コストを抑えられる。

### 出力比較

初期実装では、末尾の空白・改行差を無視する。

```text
actual.trim_end() == expected.trim_end()
```

行中の空白差や Special Judge は初期対象外とする。

## 6. インフラとの関係

```text
[API Worker]
    |
    | DB の TestCase を immutable artifact として保存
    | submissionId / artifact key を送る
    v
[Queue]
    |
    | submissionId / sourceCodeKey / testCasesKey を渡す
    v
[Consumer Worker: TypeScript]
    |
    | sourceCode / testCases を取得
    v
[Storage: local file or R2]
    |
    v
[Go judge-runner in Cloudflare Sandbox]
    |
    | 判定依頼
    v
[judge-core: Rust]
    |
    | verdict
    v
[DB]
```

### 役割の違い

| 層 | 役割 |
| --- | --- |
| インフラ | どこで動かすか、どう運ぶか |
| Consumer Worker | 判定ジョブをどう受け取り、どう返すか |
| Go judge-runner | Sandbox内でどの形式の入力を受け、どう判定結果を返すか |
| judge-core | 何をどう判定するか |

Rust の成果物はインフラの代替ではなく、インフラの上で最も重要な処理を担うアプリケーション部品である。

## 7. 開発順序

```text
Phase 1: judge-core
- ローカル CLI で直接実行できる
- Python、複数ケース、AC/WA/TLE/RE/IE

Phase 2: Consumer Worker / Go judge-runner
- Queue job を受け取る
- judge-core を呼ぶ
- 結果を DB に返す

Phase 3: API / UI と接続
- 提出から結果表示まで一気通貫にする
```

この順序により、プロジェクトの中心である「裏で動く仕組み」を最初から主役にできる。

## 8. 現時点で保留する論点

- sandbox の具体方式
- TestCase の artifact retention / garbage collection 方針
- `judge-core` を crate のみで持つか、CLI バイナリも持つか
- C++ 対応時に CE をどの粒度で扱うか
