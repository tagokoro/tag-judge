# ADR-0001: 競プロサービスを主軸に既存のGo APIとRust判定基盤を使う

- 日付: 2026-10-06
- 状態: 提案（このADRを含むPRのマージで採用）
- 対象: [#25](https://github.com/tagokoro/tag-judge/issues/25)、[全体計画 #33](https://github.com/tagokoro/tag-judge/issues/33)
- 仕様: [プロダクト設計](../competitive-programming-service.md)
- 構成図: [インフラ設計](../infrastructure-design.md)

## 背景

mainには問題管理・提出・判定結果API、Go API、PostgreSQL、S3/SQS adapter、Rust Worker、Linux isolate runnerがある。Submissionの受付から結果保存までは接続済みで、CodeRunのstdin Workerは未実装である。次の開発を競技プログラミングサービスの利用フローに集中させる。

## 決定

1. Problem・TestCase・Submissionを主要ドメインとする。CodeRunは提出前の動作確認機能とし、サンドボックスは内部の実行基盤として共通化する。
2. 既存の公開エンドポイント、整数ID、pending/runningと判定結果の契約を維持する。汎用execution APIやExecutionテーブルへの移行は行わない。
3. Go APIは認証・問題管理・受付・結果取得、Rust Workerは隔離実行・結果保存を担当する。DBはPostgreSQL、配送はSQS、artifactはS3を継続使用する。最初の検証はLocalStackで行う。
4. 実行hostはLinux/isolateとし、AWS配置ではEC2を使う。Workerだけが必要なDB・S3・SQS権限を持ち、ユーザープロセスへcredentialを渡さない。private配置と最小権限は #31 で実装する。
5. Submissionのoutboxを活かし、CodeRunにも適用する。結果はDBへ直接保存してからackし、lease・試行token・stale recoveryを #29 で追加する。別のResult Consumerは増やさない。
6. 問題閲覧・提出・結果・履歴のUIとCodeRunを優先する。追加言語はC++を先に扱う。コンテストと順位表は競プロサービスの拡張として別途仕様を定める。
7. Cloudflareへのcontrol plane移行は必要になった段階で評価する。[PR #24](https://github.com/tagokoro/tag-judge/pull/24) の構成は別の選択肢であり、このADRによって採用しない。

## 実行hostの選定

isolateはLinuxのnamespacesとcontrol groupsを利用する。現在のrunnerを継続するため、これらを設定できるLinux環境を実行先とする。[isolate公式README](https://github.com/ioi/isolate/tree/v2.6)

通常のCloudflare Workersの `node:child_process` は非機能のstubである。現在のisolateバイナリを直接起動する構成には使わず、Cloudflare APIを採用する場合も実行先を別に評価する。Containers/Sandbox等は通常のWorkersとは別の選択肢として扱う。[Workers公式資料](https://developers.cloudflare.com/workers/configuration/compatibility-flags/)

Fargateにはprivileged containerや基盤hostへのアクセスの制約がある。現行isolateを設定・検証しやすいEC2を選ぶ。API側の配置までEC2に固定する判断ではない。[AWS公式資料](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/fargate-security-considerations.html)

## 比較とトレードオフ

| 選択肢 | 判断 |
| --- | --- |
| 競プロサービス + 既存Go/Rust基盤 | 採用。問題・提出・判定APIを活かして利用者向け機能を完成させる |
| 汎用サンドボックスAPIへ転換 | 不採用。新しいexecution契約・ID移行・互換adapterを開発対象にしない |
| Cloudflare APIへ同時移行 | 保留。DB・認証・queue・結果保存の移行よりサービスの導線を優先 |
| API内でコードを直接実行 | 不採用。公開受付と未信頼コードの権限・障害境界を分離する |

問題の比較ロジックとCodeRunの実行結果は分け、runner・artifact・配送処理を共有する。Go/RustがDB契約を共有するため、schemaやjobの変更はWorkerの互換性とdeployment順序を確認する。

少なくとも1回の配送を前提とし、結果の確定を一度に制限する。物理的な重複実行を完全には防げないため、各試行の資源制限とcleanupを維持する。isolate導入だけで公開可能とはせず、#28のLinux試験と #30/#31 の利用制御・運用準備を公開条件とする。

## 再評価

ホスト運用の負担や費用、DB結合が実際の支障になった場合にproviderやResultSinkを再評価する。変更する場合は既存の問題・提出・CodeRun契約、隔離試験、配送・結果整合性を維持し、移行とrollbackを別ADRで定義する。
