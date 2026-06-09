# インフラ設計方針

## 1. 目的

このドキュメントは、Rust 製オンラインジャッジ基盤における初期インフラ設計の方針を整理する。

このプロジェクトでは、単に Web アプリケーションを公開するだけでなく、以下を設計上の重要課題として扱う。

- 提出受付と Rust 製 Judge Worker を分離する
- 提出集中を吸収する
- 判定処理の失敗に備える
- 将来の拡張余地を残しつつ、初期段階で過剰に複雑化しない

## 2. 現時点の結論

初期開発ではローカル環境を中心に進めるが、本番環境は AWS を前提に設計する。

```text
初期開発: ローカル中心
本番想定: AWS
設計方針: 本番で AWS に移しやすい責務境界を最初から作る
```

理由は、コード実行を伴う Judge Worker を API Server から分離する必要があり、AWS のマネージドサービスがその構成と相性が良いためである。

## 3. 最小構成

```text
[Frontend]
    ↓
[API Server]
    ↓        ↘
 [RDS]      [SQS]
              ↓
[Judge Worker: Rust]
              ↓
             [RDS]

必要に応じて:
- テストケースや提出コードの保存先として S3
```

### 各コンポーネントの責務

| コンポーネント | 責務 |
| --- | --- |
| Frontend | コンテスト閲覧、問題表示、提出、質問、順位表表示 |
| API Server | 認証、認可、提出受付、質問管理、順位表 API 提供 |
| RDS | User / Contest / Problem / Submission / Question などの永続データ保存 |
| SQS | `submissionId` を含む判定ジョブの待ち行列 |
| Judge Worker | Queue からのジョブ取得、提出コード・テストケース取得、`judge-core` 呼び出し、判定結果返却 |
| judge-core | Rust 製の判定エンジン。compile / run / compare を担当 |
| S3 | テストケース、提出コード、実行ログなどのファイル保存が必要になった場合に利用 |

## 4. なぜ Queue を置くのか

提出処理を API Server 内で同期的に完結させると、コード実行の負荷や失敗が API 全体へ波及する。

そこで、提出受付と判定実行の間に Queue を置く。

```text
ユーザーが提出
  ↓
API Server が Submission を保存
  ↓
Queue に判定ジョブを追加
  ↓
Judge Worker がジョブを取得
  ↓
Judge Worker が storage から sourceCode / testCases を取得
  ↓
judge-core が compile / run / compare を実行
  ↓
判定結果を返す
```

### 解決する課題

- 提出集中時でも API Server が受付に集中できる
- Judge Worker を独立して増減できる
- Worker 障害時の再試行設計を取り入れやすい
- API と Judge の責務を明確に分離できる

## 5. Redis ではなく SQS を本番 Queue に選ぶ理由

Redis は高速なインメモリデータストアであり、本番環境でも利用できる。AWS 上では ElastiCache for Redis を利用できる。

ただし、このプロジェクトにおける Queue の主目的は、キャッシュではなく **判定ジョブを安定して受け渡すこと** である。

SQS はメッセージキュー専用サービスであり、以下のような運用上の仕組みを標準で持つ。

- 処理中メッセージを一時的に隠す visibility timeout
- 失敗し続けるジョブを隔離する dead-letter queue
- Worker の増減に合わせた疎結合な処理

そのため、現時点では次の方針とする。

```text
Queue: SQS を本番想定とする
Redis: 必要性が生じるまで導入しない
```

Redis は将来的に、キャッシュやセッション管理、ランキング補助などに必要になった場合に検討する。

## 6. ローカル開発と本番環境の対応

| 役割 | ローカル開発 | 本番想定 |
| --- | --- | --- |
| API Server | ローカル実行 / コンテナ | ECS on Fargate |
| Judge Worker | Rust バイナリをローカル実行 / コンテナ | ECS on Fargate |
| judge-core | Rust crate / CLI としてローカル実行 | Judge Worker コンテナ内で利用 |
| DB | PostgreSQL | RDS for PostgreSQL |
| Queue | 抽象インターフェース + 簡易実装 | SQS |
| ファイル保存 | ローカルストレージ | S3 |

### 設計上の意図

ローカル環境と本番環境で具体的なサービスは異なっても、アプリケーション内部では以下の責務を固定する。

- `Submission` を保存する
- 判定ジョブを発行する
- Worker がジョブを取得する
- `judge-core` が実際の判定を行う
- 判定結果を返し、API Server または Result Consumer が永続化する

Queue 実装を直接アプリケーション全体へ散らさず、境界を抽象化しておくことで、本番移行時の変更範囲を抑える。

## 7. Submission の状態遷移

```text
PENDING   : 提出受付済み、まだ未処理
JUDGING   : Worker が処理中
AC        : 正解
WA        : 不正解
ERROR     : 実行基盤または想定外の障害
```

この状態遷移を持つことで、frontend は提出直後に `JUDGING` を表示でき、非同期判定という backend の都合を自然な UX に変換できる。

## 8. いま学ぶべき AWS 知識

### まず必要

1. **RDS**
   - DB を AWS に置くとはどういうことか
   - PostgreSQL をどこで動かすか
   - バックアップや接続情報をどう考えるか

2. **SQS**
   - Queue とは何か
   - API Server と Worker をどう分離するか
   - 再試行や失敗ジョブ隔離をどう扱うか

3. **ECS / Fargate**
   - API Server と Judge Worker をコンテナとしてどう動かすか
   - 1つのアプリを複数プロセスへ分けて運用する考え方

4. **S3**
   - DB とファイル保存の違い
   - テストケースや提出コードをどこへ置くか

### 後回しでよい

- VPC の細かいネットワーク設計
- Auto Scaling の高度な調整
- CloudFront / Route 53 / WAF
- Redis / ElastiCache
- IaC の高度な作り込み
- マルチ AZ や冗長化の細部

これらは重要ではあるが、現段階ではアプリケーションの核となる責務分離を理解してからでよい。

## 9. 学習順

```text
Step 1: ローカルで API / Worker / DB / Queue の役割を理解する
Step 2: RDS を理解して、DB を「自分で動かす」から「預ける」へ進む
Step 3: SQS を理解して、非同期処理を本番運用の観点で捉える
Step 4: ECS / Fargate を理解して、API と Worker を別プロセスで載せる
Step 5: 必要になったら S3 や監視、Redis を足す
```

## 10. 現時点で保留する論点

- Judge Worker / judge-core の具体的なサンドボックス方式
- 提出コードやテストケースを DB と S3 のどちらへ置くか
- Queue のローカル実装方法
- Worker の再試行回数と `ERROR` への遷移条件
- ランキングを毎回集計するか、事前計算するか

これらは backend の次段階の設計で詰める。
