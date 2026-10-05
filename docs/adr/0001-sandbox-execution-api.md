# ADR-0001: Go control planeとRust/Linux execution planeを分離する

- 日付: 2026-10-05
- 状態: 提案（このADRを含むPRのマージで採用）
- 対象: [#25](https://github.com/tagokoro/tag-judge/issues/25)、[MVP全体 #33](https://github.com/tagokoro/tag-judge/issues/33)
- 契約: [実行API・状態遷移](../sandbox-execution-api.md)
- 構成図: [インフラ設計](../infrastructure-design.md)

## 背景

mainにはGo API、PostgreSQL、S3/SQS adapter、Rust Worker、Linux isolate runnerがある。Submissionの受付・結果保存は接続されているが、CodeRunのstdin jobはWorkerで未対応である。競プロのAC/WAと汎用コード実行の成功・失敗が混在するため、プロダクトと処理境界を定義する必要がある。

当面はLocalStackで動作することを優先する。既存のGo/PostgreSQL/AWS adapterを活用すれば、provider移行と実行モデル変更を同時に行わずに受付から結果取得まで検証できる。[PR #24](https://github.com/tagokoro/tag-judge/pull/24) のCloudflare構成は別の選択肢であり、このADRでは採用しない。

## 決定

1. 公開契約はProblemに依存しない非同期execution APIとする。queued/runningと四つの終端状態を持ち、正誤比較はproblem modeだけに残す。cancelledの意味は定義するが、キャンセル操作はMVP後とする。
2. MVPのcontrol planeはGo API、永続化はPostgreSQL、配送はSQS、入力artifactはS3とする。ローカルではSQS/S3をLocalStackに置き換える。APIは認証・受付・参照を担い、ユーザーコードを実行しない。
3. execution planeはRust WorkerとLinux isolateとする。AWS配置ではLinux EC2を採用する。Workerはstorageと結果DBへ接続する信頼済みプロセスで、隔離したユーザープロセスにcredentialを継承させない。実行hostをprivate networkに配置する作業は #31 で行う。
4. DBのexecution状態を正とする。APIはexecutionとoutboxを同時commitし、Workerは結果保存後にackする。少なくとも1回の配送を前提に、leaseとattempt tokenによる条件付き更新を #29 で実装する。MVPでは別のResult Consumerサービスを増やさない。
5. Cloudflare APIは将来のcontrol plane候補とする。採用する場合も、公開API・状態遷移・認可契約を維持し、Linux execution planeとの認証済み内部接続を用意する。D1/R2/Queuesへの置換や結果callbackは別の移行設計で判断する。
6. Problem/Submissionは既存APIを維持するサンプルとする。新規利用者はexecution APIを使い、旧CodeRunは互換adapterを経由する。既存機能の削除は行わない。

## 実行hostを分ける理由

isolateはLinuxのnamespacesとcontrol groupsを利用する。そのため現在のrunnerを動かすには、これらを設定できるLinux環境が必要である。[isolate公式README](https://github.com/ioi/isolate/tree/v2.6)

通常のCloudflare Workersの `node:child_process` は非機能のstubである。このruntimeから現在のisolateバイナリを子プロセスとして起動する構成は採用できない、と判断した。Cloudflareを使う場合でも、APIの配置とコードの実行先を別々に評価する。[Workers互換性の公式資料](https://developers.cloudflare.com/workers/configuration/compatibility-flags/)

Fargateはprivileged containerと基盤hostへのアクセスを提供せず、Linux capabilityも制限する。現在のisolate設定を制御・検証しやすいEC2を実行hostに選ぶ。これはAPIのFargate配置や、別のsandbox方式の将来採用を禁止する判断ではない。[AWSのFargateセキュリティ資料](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/fargate-security-considerations.html)

## 比較した選択肢

| 選択肢 | 利点 | 費用・制約と判断 |
| --- | --- | --- |
| Go + PostgreSQL + SQS/S3 + Rust/Linux | mainの実装、LocalStack、既存の認証を活用できる | Go/Rust双方とLinux hostの運用が必要。MVPで採用 |
| Cloudflare API + Linux execution host | 公開APIの配置を独立させられる | 認証・DB・queue・結果保存の接続と移行が必要。MVP後に再評価 |
| Cloudflare Containers / Sandbox等の別実行基盤 | host管理をproviderへ寄せられる可能性がある | 通常のWorkersとは別の選択肢。権限、隔離、制限、計測、復旧の同等性を未検証のため保留 |
| APIプロセス内で直接コードを実行 | queueが不要になる | 公開受付と未信頼コードの障害・権限境界が結合するため不採用 |
| 実行結果専用のConsumer / callback API | WorkerからDBを隠せる | サービスと認証・結果配送の復旧経路が増える。現状のResultSinkで足りるため保留 |

## 結果とトレードオフ

Go APIとRust Workerが結果DBの契約を共有するため、schema migrationとdeployment順序の調整が必要になる。jobにはversionを持たせ、旧problem jobを維持しながらWorkerを先に更新する。公開APIはproviderのqueue payloadやartifact keyに依存させない。

APIの202は受付の永続化を意味し、実行成功を保証しない。配送とleaseによる復旧では物理的な重複実行が起こり得るため、結果の確定だけを一度に制限する。外部ネットワークを閉じ、各試行に資源上限を適用する。

isolate導入済みという事実だけで公開可能とはしない。MVPは #28 のLinux隔離試験、公開デモはさらに #30 の利用制御と #31 の配置・監視を必要とする。Linux kernel自体の脆弱性に対する完全な防御や、VMと同等の分離は保証しない。

## 再評価する条件

- Cloudflareへの移行が、DB・認証・結果整合性を含む運用負担や費用を実測で改善する。
- 別の実行基盤で #28 の安全性試験と #29 の障害復旧試験を満たし、時間・メモリ・終了状態を同じ契約で返せる。
- 実行hostの管理や共有DBの結合が支障になり、ResultSinkを内部APIへ置き換える必要が生じる。

いずれの場合も後続ADRで、変更対象・既存jobの排出・データ移行・rollbackを定義する。この決定だけでAWSへdeployしたり、Cloudflareの既存PRを変更したりはしない。
