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

`judge-core` は Queue、DB、AWS を知らない。これにより、ローカル CLI からも単体テストからも扱いやすくする。

### judge-worker

インフラとの接続を担う常駐プロセス。

責務:

- Queue から判定ジョブを取得する
- storage から提出コードやテストケースを取得する
- `judge-core` を呼び出す
- 判定結果を返す
- 必要に応じて失敗を再試行可能な形で扱う

現時点では、`judge-worker` は DB に直接アクセスしない方針とする。  
ローカル実装では file path を受け取り、本番想定では S3 key を受け取る。

```text
local JudgeJob
- submissionId
- language
- sourceCodePath
- testCasesPath
- timeLimitMs

production JudgeJob
- submissionId
- language
- sourceCodeKey
- testCasesKey
- timeLimitMs
```

判定結果を DB に保存する責務は、将来的には API Server または Result Consumer が担う。

## 3. なぜ分けるのか

```text
judge-core
- AWS を知らない
- Queue を知らない
- DB を知らない
- ただ判定する

judge-worker
- AWS を知る
- Queue を知る
- DB を知る
- 仕事を運ぶ
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

本番構成では、提出コードそのものは S3 に保存し、Queue には `sourceCodeKey` のような参照情報を積む。ただし、`judge-core` は S3 を知らない。`judge-worker` が S3 から取得した実際の `sourceCode` を `judge-core` に渡す。

```text
SQS job
  ↓
judge-worker
  ├─ S3 から sourceCode を取得
  ├─ S3 または DB から testCases を取得
  └─ judge-core に実データを渡す
```

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
[API Server]
    |
    | submissionId を送る
    v
[Queue]
    |
    | submissionId / sourceCodeKey / testCasesKey を渡す
    v
[judge-worker: Rust]
    |
    | sourceCode / testCases を取得
    v
[Storage: local file or S3]
    |
    v
[judge-worker: Rust]
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
| judge-worker | 判定ジョブをどう受け取り、どう返すか |
| judge-core | 何をどう判定するか |

Rust の成果物はインフラの代替ではなく、インフラの上で最も重要な処理を担うアプリケーション部品である。

## 7. 開発順序

```text
Phase 1: judge-core
- ローカル CLI で直接実行できる
- Python、複数ケース、AC/WA/TLE/RE/IE

Phase 2: judge-worker
- Queue job を受け取る
- judge-core を呼ぶ
- 結果を DB に返す

Phase 3: API / UI と接続
- 提出から結果表示まで一気通貫にする
```

この順序により、プロジェクトの中心である「裏で動く仕組み」を最初から主役にできる。

## 8. 現時点で保留する論点

- sandbox の具体方式
- テストケースと提出コードの保存先
- `judge-core` を crate のみで持つか、CLI バイナリも持つか
- C++ 対応時に CE をどの粒度で扱うか
