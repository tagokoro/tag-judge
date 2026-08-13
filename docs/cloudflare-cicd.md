# Cloudflare CI

Cloudflare runtimeのCIは、まずdeployではなく検証だけを行う。

対象:

- `cloudflare/**`
- `.github/workflows/cloudflare.yml`

CIで確認すること:

- `npm ci`
- TypeScript typecheck
- API Worker の `wrangler deploy --dry-run`
- Consumer Worker の `wrangler deploy --dry-run --containers-rollout=none`
- Go judge-runner の `go test ./...`

`--containers-rollout=none` は、CIでWorker bundleとbinding設定を検証しつつ、Sandbox containerのrolloutを行わないために使う。container image buildまで含めた確認は、Dockerが使えるローカル環境またはdeploy用CIで別途行う。

本番deployをCIへ載せるときは、次をGitHub Secretsへ設定する。

- `CLOUDFLARE_API_TOKEN`
- `CLOUDFLARE_ACCOUNT_ID`

D1の `database_id` は `wrangler d1 create tag-judge` で発行された値を `cloudflare/*/wrangler.jsonc` に反映する。
