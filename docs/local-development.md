# ローカル開発環境

## 1. 目的

ローカルでは次の2系統を使い分ける。

| 系統 | 目的 |
| --- | --- |
| Go backend + PostgreSQL | 既存APIとユースケース実装の確認 |
| Cloudflare Workers + D1/R2/Queue/Sandbox | 本番想定のCloudflare実行経路の確認 |

AWS / LocalStack はアクティブな開発前提から外す。提出コードとテストケースは Cloudflare 構成では R2、metadata と結果は D1、判定ジョブは Queue を使う。

## 2. Go backend の起動

```sh
cp .env.example .env
docker compose up -d postgres
make db-setup
make run-api
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

## 3. Cloudflare Workers の準備

```sh
cd cloudflare
npm install
```

Cloudflare resourceを作成する:

```sh
npx wrangler d1 create tag-judge
npx wrangler r2 bucket create tag-judge-artifacts
npx wrangler queues create tag-judge-jobs
npx wrangler queues create tag-judge-jobs-dlq
```

`wrangler d1 create` が返す `database_id` を次の2ファイルへ反映する。

- `cloudflare/api-worker/wrangler.jsonc`
- `cloudflare/consumer-worker/wrangler.jsonc`

D1 migration:

```sh
npm run d1:migrate:local
```

API Worker:

```sh
npm run dev:api
```

Consumer Worker:

```sh
npm run dev:consumer
```

Cloudflare Sandbox は初回起動時にDockerでcontainer imageをbuildするため、数分かかることがある。

## 4. Cloudflare版 Submission API の疎通

MVPのCloudflare API Workerは、session auth移植前の仮認証として `X-User-ID` を受け取る。

```sh
curl -i http://localhost:8787/healthz

curl -i http://localhost:8787/api/v1/problems/1/submissions \
  -H 'content-type: application/json' \
  -H 'x-user-id: dev-user-1' \
  -d '{
    "language": "python",
    "source_code": "print(input())\n"
  }'
```

このAPIは次の順に処理する。

```text
request
  -> D1 から problem / test_cases を読む
  -> R2 に source code を保存
  -> R2 に test case artifact を保存
  -> D1 に pending submission を作る
  -> Queue に judge job を送る
```

Consumer WorkerはQueue messageを受け取り、R2からartifactを読み、Cloudflare Sandbox内のGo judge-runnerを実行してD1へ結果を保存する。
