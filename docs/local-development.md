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
`judge-worker` は Queue との接続を担うが、現段階では DB スキーマには依存させない。

## 2. 構成

| サービス | ローカル | 本番想定 | 役割 |
| --- | --- | --- | --- |
| DB | PostgreSQL 16 | RDS for PostgreSQL | API が管理する永続データ |
| Queue | LocalStack SQS | Amazon SQS | 判定ジョブの受け渡し |
| Object Storage | LocalStack S3 | Amazon S3 | 提出コード、テストケース、実行ログなどの保存候補 |
| Judge Runner | Docker + isolate | 未定 | 未信頼コード実行の sandbox 検証 |

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

## 6. S3 の動作確認

LocalStack には `judge-storage` bucket を作成する。  
現時点では `judge-worker` から S3 を読む実装には接続しない。

sample file を put/get する例:

```sh
printf 'print("hello")\n' | docker compose exec -T localstack awslocal s3 cp \
  - \
  s3://judge-storage/submissions/sub_local_001/main.py

docker compose exec -T localstack awslocal s3 cp \
  s3://judge-storage/submissions/sub_local_001/main.py \
  -
```

将来 S3 を worker に接続する場合は、`JudgeJob` の `sourceCodePath` / `testCasesPath` を直接 S3 key に置き換えるのではなく、local storage と S3 storage を切り替えられる境界を設計してから進める。

## 7. isolate 実行環境の確認

`judge-runner` は、未信頼コードを安全に実行する sandbox 候補として `isolate` を検証するためのローカル Docker 環境である。

```sh
docker compose build judge-runner
docker compose up -d judge-runner
```

`isolate` が入っていることを確認する:

```sh
docker compose exec judge-runner isolate --version
docker compose exec judge-runner isolate-check-environment
```

Docker Desktop 上では `isolate-check-environment` が cgroup root、swap、kernel parameter などの警告を出す場合がある。  
これは macOS 上の Docker VM 制約によるもので、まずは `isolate --init` と `isolate --run` が動くことを確認する。

簡単な Python 実行を `isolate` 経由で確認する:

```sh
docker compose exec judge-runner sh -lc 'box=$(isolate --init); cp /workspace/judge/worker/examples/submissions/sub_ac_001/main.py "$box/main.py"; isolate --run --dir=/box="$box" -- /usr/bin/python3 /box/main.py <<EOF
hello
EOF
status=$?; isolate --cleanup; exit $status'
```

Rust の既存テストを Linux コンテナ内で実行する:

```sh
docker compose exec judge-runner cargo test --manifest-path /workspace/judge/core/Cargo.toml
docker compose exec judge-runner cargo test --manifest-path /workspace/judge/worker/Cargo.toml
```

`judge-runner` は Cargo の dependency cache と build output を Docker volume に保存する。

```text
CARGO_HOME=/cargo
CARGO_TARGET_DIR=/cargo-target
```

そのため、通常の `docker compose up -d judge-runner` や `docker compose restart judge-runner` では毎回フルコンパイルしない。  
cache を消したい場合だけ、次のように volume ごと削除する。

```sh
docker compose down -v
```

現時点では `judge-core` の runner 実装には接続しない。  
まず Docker 内で `isolate` が使える実行環境を作り、次の段階で `DirectRunner` と `IsolateRunner` を切り替えられる境界を設計する。

`judge-runner` はローカル検証用に `privileged: true` を使う。これは `isolate` が namespace / cgroup / mount まわりの Linux 機能に依存するためであり、本番構成でそのまま採用する前提ではない。

## 8. 設計上の位置づけ

この段階では DB テーブルを先に作り込まない。  
`users`, `problems`, `submissions`, `test_cases` などのスキーマは、API Server の責務、認証、認可、提出データの保存場所を設計してから追加する。

Queue は先に用意する。  
理由は、提出受付と判定実行を分離する設計境界を早い段階で確認できるためである。
