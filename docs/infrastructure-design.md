# インフラ設計方針

## 1. 目的

このドキュメントは、オンラインジャッジ基盤の本番インフラ方針を整理する。

このプロジェクトでは、単に API を公開するだけではなく、以下を設計上の重要課題として扱う。

- 提出受付と判定実行を分離する
- 提出集中を Queue で吸収する
- 提出コードとテストケースを DB へ直接詰め込まない
- 未信頼コードを API 実行環境から隔離する
- MVP では小さく始めつつ、複数言語や再ジャッジへ拡張できる境界を残す

## 2. 現時点の結論

本番想定は AWS ではなく Cloudflare Developer Platform に寄せる。

```text
Browser / Frontend
  -> API Worker: TypeScript
      -> D1: metadata / results
      -> R2: source code / test case artifacts
      -> Queue: judge jobs
  -> Consumer Worker: TypeScript
      -> Cloudflare Sandbox
      -> Go judge-runner
      -> D1: AC / WA / TLE ...
```

理由は、API、DB、object storage、queue、隔離実行環境を Cloudflare 内の binding として扱えるためである。小さいMVPでは運用対象を増やさず、提出受付から判定結果反映までを1つのプラットフォーム上で確認できる。

## 3. 最小構成

| コンポーネント | 責務 |
| --- | --- |
| Frontend | 問題閲覧、提出、提出結果閲覧 |
| API Worker | 認証、認可、提出受付、metadata作成、R2保存、Queue発行 |
| D1 | User / Problem / TestCase / Submission / Result / Session などの永続データ |
| R2 | 提出コード、テストケースartifact、将来の実行ログ |
| Queue | 判定ジョブの待ち行列 |
| Consumer Worker | Queue messageを受け取り、artifact取得、Sandbox起動、結果永続化を行う |
| Cloudflare Sandbox | 未信頼コードをAPI Workerから隔離して実行する |
| Go judge-runner | Sandbox内で source / testcases を受け取り、AC / WA / TLE などを返す |

## 4. Queue を置く理由

提出処理を API Worker 内で同期的に完結させると、コード実行の負荷や失敗が API 全体へ波及する。

そこで、提出受付と判定実行の間に Queue を置く。

```text
ユーザーが提出
  -> API Worker が Submission を pending で保存
  -> API Worker が source code / testcases を R2 に保存
  -> API Worker が Queue に judge job を送る
  -> Consumer Worker が job を受け取る
  -> Consumer Worker が R2 から artifact を読む
  -> Cloudflare Sandbox 内の Go judge-runner が判定する
  -> Consumer Worker が D1 へ結果を保存する
```

これにより、提出集中時でも API Worker は受付に集中でき、判定側は Queue の再配送を前提に冪等に処理できる。

## 5. D1 / R2 / Queue の使い分け

| データ | 保存先 | 理由 |
| --- | --- | --- |
| User / Session / Problem / Submission / Result | D1 | 検索、一覧、状態更新、参照整合性が必要 |
| 提出コード | R2 | 本文が大きくなりやすく、DB行に直接置かない方が扱いやすい |
| テストケースartifact | R2 | 提出受付時点の不変snapshotとして参照する |
| 判定ジョブ | Queue | APIと判定処理を非同期に分離する |

Submission は `source_code_key` と `test_cases_key` だけを持つ。実体は R2 に保存する。

## 6. クリーンアーキテクチャ上の境界

既存Go backendでは usecase が `ArtifactStorage` と `ExecutionDispatcher` interface に依存しており、S3/SQSなどの具体サービスを知らない。この境界はCloudflare版でも維持する。

Cloudflare版では、具体実装は次のように外側へ置く。

- API Worker: R2へartifact保存、Queueへjob送信
- Consumer Worker: R2からartifact取得、Sandboxへ実行依頼、D1へ結果保存
- Go judge-runner: DB / R2 / Queueを知らず、source / testcases / config から判定結果だけを返す

## 7. MVP の制約

- 対応言語は Python のみ
- CodeRun は任意stdin実行として後続でCloudflare版へ移植する
- scoring は全ケースACで満点、1ケースでも非ACなら0点
- 認証は最終的に session cookie を Worker 側へ移植する
- Cloudflare Sandbox はPaid plan前提のため、ローカル確認にはDockerが必要

## 8. 次に詰める論点

- Google OAuth callback を API Worker に移すか、Go backendを認証だけ暫定併用するか
- Session token hash を D1 に置く運用で問題ないか
- D1 schema と既存PostgreSQL migrationをどのタイミングで一本化するか
- CodeRun API を Submission と同じ Queue / Sandbox 経路に乗せるか
- 管理者向けの再ジャッジを Queue message としてどう表現するか
