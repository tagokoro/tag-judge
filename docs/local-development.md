# ローカル開発環境

## 1. 目的

この Docker 環境は、アプリケーション全体をコンテナ化するためではなく、ローカル開発で必要な外部依存を再現するために用意する。

```text
Rust crates: ローカルで実行
PostgreSQL: Docker
SQS: LocalStack
```

`judge-core` は DB / Queue / AWS を知らない判定エンジンとして保つ。  
`judge-worker` は Queue との接続を担うが、現段階では DB スキーマには依存させない。

## 2. 構成

| サービス | ローカル | 本番想定 | 役割 |
| --- | --- | --- | --- |
| DB | PostgreSQL 16 | RDS for PostgreSQL | API が管理する永続データ |
| Queue | LocalStack SQS | Amazon SQS | 判定ジョブの受け渡し |

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
```

## 5. Judge Worker の実行

`judge-worker` は LocalStack SQS の `judge-submissions` から `JudgeJob` を 1 件受信し、`judge-core` で判定して、処理に成功したら SQS message を削除する。

```sh
cd judge-worker
cargo run
```

LocalStack へ投入する message body は `JudgeJob` の JSON とする。

```json
{
  "submissionId": "sub_local_001",
  "language": "python",
  "sourceCodePath": "/absolute/path/to/main.py",
  "testCasesPath": "/absolute/path/to/testcases.json",
  "timeLimitMs": 1000
}
```

現時点では判定結果は標準出力に出す。  
API Server と DB の設計後に、結果の永続化責務を API Server または Result Consumer に接続する。

## 6. 設計上の位置づけ

この段階では DB テーブルを先に作り込まない。  
`users`, `problems`, `submissions`, `test_cases` などのスキーマは、API Server の責務、認証、認可、提出データの保存場所を設計してから追加する。

Queue は先に用意する。  
理由は、提出受付と判定実行を分離する設計境界を早い段階で確認できるためである。
