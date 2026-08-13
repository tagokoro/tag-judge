# Cloudflare runtime

This directory contains the Cloudflare target runtime for Tag Judge.

```text
Browser / Frontend
  -> API Worker
      -> D1 for metadata and judge results
      -> R2 for source code and test case artifacts
      -> Queue for judge jobs
  -> Consumer Worker
      -> Cloudflare Sandbox
      -> Go judge-runner
      -> D1 result update
```

## Projects

| Path | Role |
| --- | --- |
| `api-worker` | HTTP API. It accepts submissions, stores artifacts in R2, creates metadata in D1, and sends judge jobs to Queue. |
| `consumer-worker` | Queue consumer. It reads artifacts from R2, runs the Go judge-runner in Cloudflare Sandbox, and writes results to D1. |
| `consumer-worker/judge-runner` | Go runner executed inside the Sandbox container. MVP supports Python only. |
| `d1/migrations` | D1 schema. This mirrors the current backend schema using SQLite-compatible types. |

## First setup

Install dependencies:

```sh
cd cloudflare
npm install
```

Create Cloudflare resources:

```sh
npx wrangler d1 create tag-judge
npx wrangler r2 bucket create tag-judge-artifacts
npx wrangler queues create tag-judge-jobs
npx wrangler queues create tag-judge-jobs-dlq
```

Replace the placeholder `database_id` in both `api-worker/wrangler.jsonc` and
`consumer-worker/wrangler.jsonc` with the D1 ID returned by `wrangler d1 create`.

Apply D1 migrations:

```sh
npm run d1:migrate:local
```

Run workers locally:

```sh
npm run dev:api
npm run dev:consumer
```

The first Sandbox run builds the container image, so it can take a few minutes.

## Current MVP scope

- `POST /api/v1/problems/{problem_id}/submissions`
- Python only
- Public problems only
- Temporary `X-User-ID` header auth until session auth is ported to Workers
- All cases AC means full score; any non-AC case makes the total score 0

The current Go backend is still useful as the reference implementation while API
routes are moved to Workers one by one.
