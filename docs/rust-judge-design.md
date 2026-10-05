# Rust Workerとjudge-coreの設計

Rustは競プロサービスの問題判定と、CodeRunの動作確認を担当する。[プロダクト設計](competitive-programming-service.md) を基準とし、Problem/Submissionを主要機能として維持する。本書の「現行」はmain `f8f5e0c` 時点を示す。

## 責務

| コンポーネント | 責務 |
| --- | --- |
| Go API | 認証・問題へのアクセス、制限値決定、artifact保存、受付とoutbox、結果取得 |
| judge-worker | queue受信、開始記録、artifact取得、core呼び出し、DBへの結果保存、保存後ack |
| judge-core | ケース実行、出力比較、verdict集約。stdin modeでは比較せず一度実行 |
| runner | 隔離環境の準備、子プロセス起動、制限適用、入出力と計測値の回収、cleanup |

WorkerはPostgreSQLへ直接結果を保存する。core/runnerにはDB・SQS・S3の責務を持たせない。CLIはcoreを単独検証する入口として維持する。構成図は [インフラ設計](infrastructure-design.md) を参照する。

## 現行のproblem mode

API側の `ExecutionJob` はreference（submission/code_run）とexecutionMode（problem/stdin）を持つ。現行Rust Workerの `JudgeJob` は次のSubmission向けフィールドを読むため、APIはproblem jobに互換フィールドも含める。

```text
submissionId
language
sourceCodeKey
testCasesKey
timeLimitMs
```

APIはSubmissionのソースを保存し、公開済みのテストケーススナップショットへの参照を確定する。Submissionとoutboxを同じDB transactionで作り、publisherがSQSへ配送する。Workerがsourceとスナップショットを取得してcoreへ渡す。

coreはPythonをケースごとに実行し、末尾の空白・改行を除いた出力を比較する。最初の非ACで止まり、実行済みケースだけを返す。Pythonの構文エラーや例外はRE。全ケースACの得点はケース得点の合計、それ以外は0。詳細は [judge-core要件](judge-core-requirements.md) に従う。

Workerは `submissions` と `submission_results` へ判定・ケース結果・得点を一つのtransactionで保存し、その後にackする。DB/S3の一時障害は再配送し、終端Submissionは再判定しない。visibilityを判定中に延長する。現在の行ロックと終端チェックだけでは実行開始の排他は完成しておらず、#29でleaseと試行tokenを追加する。

## CodeRun / stdin mode

CodeRunは自分のソースと入力を試す機能として既存APIを維持する。現行APIはpendingレコードを作り、ソースをS3へ保存し、stdin jobを直接dispatchする。失敗時にpendingをrollbackせず、Rust Workerもstdin jobを処理できない。

#27ではproblem/stdinを判別し、stdin modeにtest casesを要求せず一度実行する。開始時はcode_runsをrunningへ更新し、正常終了ac、非0終了re、timeoutはtle、基盤障害はieとして既存の結果フィールドに保存する。acは正解判定ではなく正常終了を表す。

終了コード・計測値・内部の失敗理由をrunnerから区別できるようにし、必要なAPI拡張は既存契約を保って追加する。DB保存後にackし、再配送で完了結果を壊さない。新しい公開executionモデルやCodeRun IDの移行は行わない。

#29ではCodeRunにもoutboxと復旧を適用する。冪等性の単位はsubmission/code_runの種別とIDであり、同じ整数IDの異なる種別を混同しない。終端状態は不変とし、古い試行の結果を拒否する。目標遷移は [プロダクト設計](competitive-programming-service.md#4-状態遷移と再配送) を参照する。

## 制限と追加言語

現行isolate runnerはinit/run/cleanupとboxの排他を扱う。CPU時間は要求値、wall clockは要求値に1秒を加えた値、memoryは固定値を使う。Problemのmemory_limit_kbはrunnerへ接続されておらず、使用量も結果に保存していない。stdout/stderrの明示的なサイズ制限も未完成である。

#28でProblem/CodeRunから確定した制限をrunnerへ渡し、memory・output・process/thread・生成ファイルの制限、環境変数allowlist、filesystemの限定、外部network禁止を検証する。未計測値を0で埋めず、MLE等は確認できた制限超過に基づいて返す。CodeRunの既存statusにない制限違反はREとし、必要なら互換性のある理由フィールドで区別する。

ユーザーコードの停止とcleanupを確認してから通常の結果を保存する。Worker死亡時の復旧は #29、host再起動時の残留処理と運用は #31 につなぐ。non-Linuxのdirect runnerは信頼できる開発コードに限る。

追加言語はC++（#5）を優先し、Rust（#4）を続ける。compileも隔離し、compile timeout・生成物・CEとruntime errorを区別する。言語追加で既存Pythonの判定規則を変えない。

## 保持する境界

- 解答者にS3 key、test case参照、Problemの実行制限を指定させない。
- Workerのcredentialを子プロセスへ渡さない。
- Submission詳細に隠しテストの本文やstdout/stderrを返さない。
- stdin modeに問題の正誤比較・得点を持ち込まない。
- 既存の問題判定・得点・ケース詳細を回帰テストで維持する。

技術選定とCloudflare APIとの将来の関係は [ADR-0001](adr/0001-competitive-programming-service.md) に記録する。
