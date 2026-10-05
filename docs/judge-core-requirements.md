# judge-core 要件定義

この文書は既存のProblem/Submissionサンプルにおけるproblem modeの初期要件を記録する。汎用stdin executionの契約と現在のMVP範囲は [共通実行モデル](sandbox-execution-api.md)、Rustの実装状況は [Rust設計](rust-judge-design.md) を参照する。後述の「初期実装でやらないこと」は当初のcore単体開発の範囲であり、現在のプロダクトからメモリ制限やsandbox検証を除外する意味ではない。

## 1. 目的

`judge-core` は、提出コードと複数テストケースを受け取り、オンラインジャッジとして必要な判定結果を返す Rust 製の判定エンジンである。

このコンポーネントは、Queue / DB / S3 / AWS を直接扱わない。  
外部インフラとの接続は `judge-worker` の責務とし、`judge-core` は純粋に判定処理へ集中する。

```text
judge-worker
  ├─ sourceCode を取得する
  ├─ testCases を取得する
  └─ judge-core に渡す

judge-core
  ├─ 実行する
  ├─ 出力を比較する
  └─ verdict を返す
```

## 2. MVP で満たすべきこと

### 対応範囲

- 言語はまず Python のみ
- 標準入力 / 標準出力の通常問題のみ
- 複数テストケース対応
- 1ケースごとの実行時間制限
- 末尾の空白・改行差を無視した出力比較
- CLI から単体実行できること

### 判定できる verdict

```text
AC  : Accepted
WA  : Wrong Answer
TLE : Time Limit Exceeded
RE  : Runtime Error
IE  : Internal Error
```

`CE` は将来 C++ などのコンパイル言語を追加するときに扱う。  
Python の構文エラーや例外は、MVP では `RE` として扱う。

## 3. 入力仕様

### JudgeRequest

```text
JudgeRequest
- language
- sourceCode
- timeLimitMs
- testCases[]
```

| フィールド | 必須 | 説明 |
| --- | --- | --- |
| `language` | yes | MVP では `python` のみ |
| `sourceCode` | yes | 実行対象の提出コード |
| `timeLimitMs` | yes | 1ケースあたりの制限時間 |
| `testCases` | yes | 入力と期待出力の配列。1件以上 |

### TestCase

```text
TestCase
- input
- expectedOutput
```

| フィールド | 必須 | 説明 |
| --- | --- | --- |
| `input` | yes | 標準入力へ渡す文字列 |
| `expectedOutput` | yes | 期待される標準出力 |

### 入力 JSON 例

```json
{
  "language": "python",
  "sourceCode": "print(sum(map(int, input().split())))",
  "timeLimitMs": 1000,
  "testCases": [
    { "input": "1 2\n", "expectedOutput": "3\n" },
    { "input": "10 20\n", "expectedOutput": "30\n" }
  ]
}
```

## 4. 出力仕様

### JudgeResult

```text
JudgeResult
- verdict
- caseResults[]
- message?
```

| フィールド | 説明 |
| --- | --- |
| `verdict` | 全体の判定結果 |
| `caseResults` | 実行したケースごとの判定結果 |
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

| フィールド | 説明 |
| --- | --- |
| `caseIndex` | 0 始まりのケース番号 |
| `verdict` | そのケースの判定結果 |
| `runtimeMs` | 実行時間 |
| `stdout` | 実際の標準出力 |
| `stderr` | 実際の標準エラー |
| `message` | 補足メッセージ |

### 出力 JSON 例

```json
{
  "verdict": "AC",
  "caseResults": [
    {
      "caseIndex": 0,
      "verdict": "AC",
      "runtimeMs": 20,
      "stdout": "3\n",
      "stderr": "",
      "message": "Accepted on case 0"
    }
  ],
  "message": "Accepted"
}
```

## 5. 判定ルール

### ケースごとの判定

1. 実行時間が `timeLimitMs` を超えたら `TLE`
2. プロセスが非 0 終了したら `RE`
3. 標準出力が期待出力と一致しなければ `WA`
4. 一致すれば `AC`
5. judge-core 自体の失敗は `IE`

### 出力比較

初期実装では、末尾の空白・改行差のみ無視する。

```text
actual.trim_end() == expected.trim_end()
```

行中の空白差は区別する。

```text
"1  2" != "1 2"
```

### 複数ケースの扱い

MVP では、最初の非 `AC` ケースで実行を止める。

```text
case 0: AC
case 1: WA
case 2: 未実行

overall verdict: WA
```

すべてのケースが `AC` の場合のみ、全体 verdict を `AC` とする。

## 6. CLI 要件

`judge-core` は API Server や judge-worker が未完成でも単体検証できるよう、CLI を持つ。

```text
judge-core <request.json>
```

### CLI の責務

- JSON ファイルを読む
- `JudgeRequest` として解釈する
- 判定を実行する
- `JudgeResult` を JSON として標準出力へ出す

CLI は開発・デバッグ用の入口であり、本番では `judge-worker` からライブラリとして呼ぶ想定とする。

## 7. 推奨プロジェクト構成

```text
judge/
└─ core/
   ├─ Cargo.toml
   └─ src/
      ├─ lib.rs
      ├─ main.rs
      ├─ models.rs
      ├─ verdict.rs
      ├─ judge.rs
      ├─ compare.rs
      ├─ error.rs
      └─ runner/
         ├─ mod.rs
         ├─ direct/
         │  ├─ mod.rs
         │  └─ python.rs
         └─ isolate/
            ├─ mod.rs
            └─ python.rs
```

### 各ファイルの責務

| ファイル | 責務 |
| --- | --- |
| `lib.rs` | library として公開する入口 |
| `main.rs` | CLI の入口 |
| `models.rs` | `JudgeRequest`, `JudgeResult`, `TestCase`, `CaseResult` |
| `verdict.rs` | `AC`, `WA`, `TLE`, `RE`, `CE`, `IE` |
| `judge.rs` | 複数ケースを回し、全体 verdict を決める |
| `compare.rs` | 出力比較 |
| `error.rs` | judge-core 内部エラー |
| `runner/mod.rs` | 言語 runner の共通インターフェース |
| `runner/direct/python.rs` | direct 実行時の Python 実行処理 |
| `runner/isolate/python.rs` | isolate 実行時の Python 実行処理 |

## 8. 実装順序

自分で実装するときは、以下の順で進めるとよい。

```text
1. Cargo project を作る
2. models / verdict を定義する
3. compare を実装して unit test を書く
4. Python の sourceCode を一時ファイルへ書く
5. Python プロセスへ stdin を渡し、stdout / stderr / exit code を取得する
6. timeLimitMs を超えたら process を kill する
7. 1ケースの verdict を返す
8. 複数ケースを順番に実行する
9. 最初の非 AC で止める
10. CLI から JSON 入出力できるようにする
```

## 9. 動作確認すべきケース

最低限、以下のケースを確認する。

| ケース | 期待 verdict |
| --- | --- |
| 正しい出力 | `AC` |
| 出力が違う | `WA` |
| 無限ループ | `TLE` |
| Python 例外 | `RE` |
| `testCases` が空 | `IE` |
| 1ケース目 AC、2ケース目 WA | 全体 `WA`、3ケース目以降は未実行 |

## 10. 初期実装でやらないこと

- C++ 対応
- コンパイル処理
- 厳密なメモリ制限
- sandbox の完成形
- Special Judge
- 部分点
- Queue / DB / S3 接続
- Web API 接続

これらは judge-core の最小核が動いた後に、別フェーズで設計する。
