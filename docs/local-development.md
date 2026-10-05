# ローカル開発環境

## 1. 目的

この Docker 環境は、アプリケーション全体をコンテナ化するためではなく、ローカル開発で必要な外部依存を再現するために用意する。

```text
Rust crates: ローカルで実行
PostgreSQL: Docker
SQS: LocalStack
S3: LocalStack
```

`judge-core` は DB / Queue / AWS を知らない判定エンジンとして保つ。  
`judge-worker` は Queue / S3 との接続と、PostgreSQLへの判定結果保存を担う。

この手順は現在実装済みのproblem modeを対象とする。競プロサービスの導線とCodeRunの目標仕様は [プロダクト設計](competitive-programming-service.md) を参照する。未信頼コードの隔離検証はLinux上で行い、非Linux環境のdirect runnerには信頼できる開発用コードだけを渡す。

## 2. 構成

| サービス | ローカル | 本番想定 | 役割 |
| --- | --- | --- | --- |
| DB | PostgreSQL 16 | RDS for PostgreSQL | API が管理する永続データ |
| Queue | LocalStack SQS | Amazon SQS | 判定ジョブの受け渡し |
| Object Storage | LocalStack S3 | Amazon S3 | 提出コード、テストケース、実行ログなどの保存候補 |

Redis は現時点では導入しない。判定ジョブの受け渡しは SQS を本番前提に設計し、キャッシュやセッション管理が必要になった段階で Redis を検討する。

## 3. 起動

```sh
cp .env.example .env
docker compose up -d
```

状態確認:

```sh
docker compose ps
```

停止:

```sh
docker compose down
```

DB のデータも消す場合:

```sh
docker compose down -v
```

## 4. 接続情報

PostgreSQL:

```text
host: localhost
port: 5432
database: judge
user: judge_user
password: judge_pass
```

LocalStack:

```text
endpoint: http://localhost:4566
region: ap-northeast-1
queue: judge-submissions
dead-letter queue: judge-submissions-dlq
bucket: judge-storage
```

## 5. Judge Worker の実行

API Server は `JUDGE_STORAGE_BUCKET` と `JUDGE_SUBMISSIONS_QUEUE_NAME` が両方設定されている場合、Submission / CodeRun のソースコードを S3 に保存する。Submission はテストケースも提出時点の内容でS3へスナップショットする。
どちらかが未設定の場合は、ローカルファイル保存とログ出力 dispatcher を使うため、AWS / LocalStack なしでも API Server は起動できる。

LocalStack に接続する場合は、API Server と Worker の両方で同じ設定を使う。

```sh
export AWS_DEFAULT_REGION=ap-northeast-1
export AWS_ENDPOINT_URL=http://localhost:4566
export JUDGE_SUBMISSIONS_QUEUE_NAME=judge-submissions
export JUDGE_STORAGE_BUCKET=judge-storage
```

`judge-worker` は LocalStack SQS の `judge-submissions` から problem execution job を 1 件受信し、LocalStack S3 から提出コードとテストケースの**提出時スナップショット**を取得して `judge-core` で判定する。Worker は PostgreSQL へ判定・ケース別結果・得点を一つのトランザクションで保存してから SQS message を削除する。DB/S3 障害時は削除しないため SQS が再配信する。長時間の判定中は10秒ごとにSQSのVisibility Timeoutを延長する。既に終端状態の submission は冪等にスキップして削除する。

Submission を作成するトランザクションは、同じDB内の `submission_job_outbox` にもジョブを記録する。API内のpublisherが未送信レコードをSQSへ送信し、成功した後にのみ `dispatched_at` を更新する。SQS一時障害は指数バックオフで再試行するため、DB登録成功とSQS送信の間で提出が失われない。SQS送信成功直後にプロセスが停止した場合は重複送信され得るが、Workerの終端状態チェックとケース結果の一意制約により安全である。

ローカルで LocalStack に接続する場合は、`AWS_ENDPOINT_URL` を明示する。  
`AWS_ENDPOINT_URL` が未設定の場合、worker は AWS 本番の SQS / S3 endpoint に接続する。

```sh
cd judge/worker
export AWS_DEFAULT_REGION=ap-northeast-1
export AWS_ENDPOINT_URL=http://localhost:4566
export JUDGE_SUBMISSIONS_QUEUE_NAME=judge-submissions
export JUDGE_DATABASE_URL='postgres://judge_user:judge_pass@localhost:5432/judge?sslmode=disable'
export JUDGE_STORAGE_BUCKET=judge-storage
cargo run
```

API と Worker は必ず LocalStack を明示指定する（実 AWS へフォールバックさせない）。

```sh
export AWS_DEFAULT_REGION=ap-northeast-1
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_ENDPOINT_URL=http://localhost:4566
export JUDGE_STORAGE_BUCKET=judge-storage
export JUDGE_SUBMISSIONS_QUEUE_NAME=judge-submissions
```

### 結果と配送の確認

認証済みで `POST /api/v1/problems/{id}/submissions` を実行後、Worker を起動し、`GET /api/v1/submissions/{submission_id}` の `status` が `pending` から `ac`、`wa`、`tle`、`re`、`ce`、`ie` のいずれかへ変わり、`score` と `judged_at` が更新されることを確認する。Worker を停止して S3 または PostgreSQL を一時的に到達不能にした場合、SQS メッセージは削除されず visibility timeout 後に再配信される。

同じメッセージをもう一度送っても、終端状態の提出は再判定されずケース別結果も重複しない。`submission_results` の `(submission_id, test_case_id)` 一意制約と、Worker の行ロックがこの保証を担う。

### 提出結果の詳細API

提出者本人または管理者は、既存の詳細取得APIでケース別結果も取得できる。

```sh
curl -s -b cookies.txt "http://localhost:8080/api/v1/submissions/${SUBMISSION_ID}"
```

レスポンスの `data` は従来の提出情報に加えて、次のような `case_results` を持つ。

```json
{
  "case_results": [
    {"test_case_id": 1, "status": "ac", "execution_time_ms": 12, "memory_kb": null},
    {"test_case_id": 2, "status": "wa", "execution_time_ms": 8, "memory_kb": null}
  ]
}
```

- `pending` / `running` は `case_results: []` を返す。完了まで同じAPIをポーリングする。
- 完了後は保存済みの実行ケースだけを `test_case_id` 昇順で返す。最初の失敗で判定が止まった場合、未実行ケースは含まれない。ケース実行前のIEも空配列になる。
- 未計測の実行時間・メモリ使用量は `null`。現在のWorkerはメモリ使用量を保存しないため `memory_kb` は `null` となる。
- 隠しテストを保護するため、入力・期待出力・標準出力・標準エラーは管理者にもこのAPIからは返さない。
- 他人の提出は管理者以外 `403`、未認証は `401`、存在しない提出は `404`。提出一覧・提出作成レスポンスの形式は従来どおり。

詳細APIのPostgreSQL統合テストは、ローカルのテスト用DBを指定して実行できる。テスト専用schemaへmigrationとfixtureを投入し、終了時にschemaを削除する。指定ユーザーにはschema・extension作成権限が必要。CIではPostgreSQL 16の専用サービスで毎回実行する。

```sh
cd backend
SUBMISSION_DETAILS_TEST_DATABASE_URL='postgres://judge_user:judge_pass@localhost:5432/judge?sslmode=disable' \
  go test ./internal/infra/postgres -run TestSubmissionDetailsPostgres -v
```

LocalStack へ投入する message body は `ExecutionJob` の JSON とする。problem mode は既存workerとの互換フィールドも含める。

```json
{
  "reference": {
    "kind": "submission",
    "id": "sub_local_001"
  },
  "executionMode": "problem",
  "submissionId": "sub_local_001",
  "language": "python",
  "sourceCodeKey": "submissions/sub_local_001/main.py",
  "testCasesKey": "problems/prob_001/testcases.json",
  "timeLimitMs": 1000,
  "problemExecution": {
    "testCasesKey": "problems/prob_001/testcases.json",
    "timeLimitMs": 1000
  }
}
```

判定結果は標準出力にも出すが、PostgreSQLの `submissions` と `submission_results` が正の結果の永続先である。

## 6. S3 の動作確認

LocalStack には `judge-storage` bucket を作成する。  
`judge-worker` は `JUDGE_STORAGE_BUCKET` で指定された bucket から `sourceCodeKey` と `testCasesKey` を読む。

sample object を put/get する例:

```sh
printf 'print("hello")\n' | sudo docker compose exec -T localstack awslocal s3 cp \
  - \
  s3://judge-storage/submissions/sub_local_001/main.py

printf '[{"input":"hello\n","expectedOutput":"hello\n"}]\n' | \
  sudo docker compose exec -T localstack awslocal s3 cp \
  - \
  s3://judge-storage/problems/prob_001/testcases.json

sudo docker compose exec -T localstack awslocal s3 cp \
  s3://judge-storage/submissions/sub_local_001/main.py \
  -
```

SQS へメッセージを投入する例:

```sh
sudo docker compose exec -T localstack awslocal sqs send-message \
  --queue-url http://localhost:4566/000000000000/judge-submissions \
  --message-body '{
    "submissionId": "sub_local_001",
    "language": "python",
    "sourceCodeKey": "submissions/sub_local_001/main.py",
    "testCasesKey": "problems/prob_001/testcases.json",
    "timeLimitMs": 1000
  }'
```

`cargo run` した `judge-worker` が上記メッセージを受信すると、標準出力に判定結果 JSON を出す。

## 7. 設計上の位置づけ

現行のAPIとWorkerはPostgreSQLのschemaを共有し、Submissionの受付から結果保存までを接続している。CodeRunはAPIによるstdin jobの生成までで、Workerのstdin処理と結果保存は #27 の対象である。

今後は [プロダクト設計](competitive-programming-service.md) に従い、Problem/Submissionを中心に問題閲覧・提出・結果の画面とCodeRunの動作確認を完成させる。最新の実装状況と構成図は [インフラ設計](infrastructure-design.md) を参照する。
