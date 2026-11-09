# Library Sharing MVP Implementation Plan

> 設計: [docs/superpowers/specs/2026-11-08-library-sharing-mvp-design.md](../specs/2026-11-08-library-sharing-mvp-design.md)

## Goal and delivery order

Library を namespace 配下に保存し、immutable version と adapter を既存の Submission/Outbox/Judge で検証する。変更は依存順に積む、独立してレビュー可能な次の 4 PR とする。各 PR は前の PR を base にし、PR 間でコミットを混ぜない。

1. **PR1 — backend DB/schema/persistence**
2. **PR2 — API/usecase/handler/OpenAPI/existing judge integration**（PR1 の後）
3. **PR3 — Next.js frontend environment setup**（PR2 の後）
4. **PR4 — Next.js UI implementation**（PR3 の後）

Worker/core、既存提出 API、既存提出スキーマ、認証実装は変更しない。実装は各 PR と各 step で RED-GREEN-REFACTOR とする。

## Shared constraints

- Go 1.26、chi、pgx、sqlc、PostgreSQL 16、既存 storage/SQS、Python 3。
- source と adapter は UTF-8 1–100000 bytes、NUL/blank を拒否。hash、base64 結合テンプレート、outbox の既存 5 キーは設計書どおり。
- private library は admin を含む非所有者に存在を漏らさず 404。public の読み取り/verify は許可し、書き込みは owner のみ。
- `judge/core/**`、`judge/worker/**`、`docs/library-sharing-judge-requirements.md`、既存 submission schema/query は変更しない。
- `SUBMISSION_DETAILS_TEST_DATABASE_URL` を PostgreSQL integration test に使い、未設定時は skip する。ただし CI/受入れでは skip を成功扱いにしない。
- PR3/4 の frontend は Node.js の Next.js 公式最小要件を採用し、`create-next-app@latest` 実行時に解決された package version と lockfile を固定する。

## PR1: backend DB/schema/persistence

### Files

Create:

- `backend/db/migrations/000009_create_library_sharing.up.sql`
- `backend/db/migrations/000009_create_library_sharing.down.sql`
- `backend/db/queries/namespaces.sql`
- `backend/db/queries/libraries.sql`
- `backend/db/queries/library_versions.sql`
- `backend/db/queries/library_verifications.sql`
- `backend/internal/domain/library_sharing.go`
- `backend/internal/repository/library_sharing_repository.go`
- `backend/internal/infra/postgres/library_sharing_repository.go`
- `backend/internal/infra/postgres/library_sharing_integration_test.go`

Generated: `backend/internal/infra/postgres/sqlc/`（sqlc のみ）。PR1 では router/handler/usecase/OpenAPI/main を変更しない。

PR1 は origin/main の `000007_create_admission_control`、`000008_create_code_run_job_outbox` と admission-control 実装を保持し、library-sharing はその次の `000009` として追加する。

### Interfaces and persistence contract

- Domain types: `Namespace`, `Library`, `LibraryVersion`, `LibraryVerification`, `LibraryVisibility`。
- Repository interfaces: namespace create/find; library create/update/find/list; version create/find/list; verification transactional create/find/list。
- Version create は library row `FOR UPDATE` + `MAX(version_number)+1` の SQL で連番を割り当てる。
- `library_versions` update/delete trigger は `library_versions are immutable` で拒否する。
- verification は `submissions.problem_id/user_id` を正とし、同じ transaction で submission、既存 outbox payload、verification を保存する repository API を後続 PR に提供する。
- transaction 内で library visibility と version 所属を `FOR SHARE` 再確認し、private 化競合時は全行を rollback する。

### TDD steps and tests

1. **RED:** migration integration test を先に作る。trigger mutation rejection、sha/byte constraints、down/up を検証する。
2. **GREEN:** migration と sqlc query、domain/repository、Postgres implementation を追加する。
3. **RED/GREEN:** repository integration tests を追加・通過させる。
   - 同一 library の concurrent version create が重複なしの `1,2`。
   - namespace/library unique constraint の mapping source が取得できる。
   - verification の outbox payload が `submissionId`, `language`, `sourceCodeKey`, `testCasesKey`, `timeLimitMs` のみ。
   - verification insert failure が submission/outbox/verification を全 rollback。
   - visibility 再確認失敗が `LIBRARY_NOT_FOUND` となり行を残さない。
4. `sqlc generate` と `go test ./internal/infra/postgres -count=1`（DB URL 必須）を実行し、generated diff を同じ PR に含める。

## PR2: API/usecase/handler/OpenAPI/existing judge integration

### Files

Create:

- `backend/internal/librarycombine/combine.go`
- `backend/internal/librarycombine/combine_test.go`
- `backend/internal/usecase/namespace_usecase.go`
- `backend/internal/usecase/library_usecase.go`
- `backend/internal/usecase/library_version_usecase.go`
- `backend/internal/usecase/library_verification_usecase.go`
- `backend/internal/usecase/library_sharing_usecase_test.go`
- `backend/internal/handler/library_sharing_handler.go`
- `backend/internal/handler/library_sharing_handler_test.go`

Modify:

- `backend/internal/router/router.go`, `backend/internal/router/router_test.go`
- `backend/cmd/api/main.go`
- `backend/docs/openapi.yaml`

### Interfaces and behavior

- `librarycombine.ValidateSource`, `ContentSHA256`, `Combine`。Python は実行せず、設計書の base64 template を生成する。
- Namespace/Library/Version/Verification usecase は PR1 repository を受け取り、設計書の認可、validation、pagination、error code/message を実装する。
- chi routes は設計書 §10 の全 namespace/library/version/verification paths。PATCH/DELETE version は未登録で 405。
- verify は `FindPublicByID`、既存 storage、`submissionOutboxPayload`、既存 submission/outbox transaction を使う。新 queue/worker message/runner は作らず、`SubmissionUsecase.Create` を呼ばない。
- OpenAPI は API 契約を網羅する。レスポンス作成時は adapter/case results を隠し、開始者/admin の詳細だけに返す。

### TDD steps and tests

1. **RED:** combine fixture の byte/hash/template 一致と `python3` stdout `3\n`、source boundary tests を作る。
2. **GREEN:** combiner を実装する。
3. **RED/GREEN:** usecase tests を実装する。namespace slug/name、library visibility、private-admin 404、version immutable/no update、pagination、invalid adapter/problem/version を固定する。
4. **RED/GREEN:** handler/router tests を実装する。JSON shape、unknown fields、ID overflow、version PATCH/DELETE 405、adapter/source 非公開を固定する。
5. **RED/GREEN:** verification integration/API tests を実装する。public verify、private admin rejection、no test cases 409、outbox 5 keys、transaction rollback、visibility race、case result polling shape、既存 submission compatibility を固定する。
6. `sqlc generate && go test ./...`、Rust の既存 test、`git diff -- judge/core judge/worker`（空）を確認する。

## PR3: Next.js frontend environment setup

### Files

Create under `frontend/`:

- `package.json`, package lock（npm の resolved lockfile）
- `tsconfig.json`, `eslint.config.*`, `next.config.*`
- `app/layout.tsx`, `app/page.tsx`（placeholder）
- `.env.example` or documented env template

Modify only setup/docs/local targets:

- `Makefile`, `.env.example`, `README.md`
- Go 側の `backend/internal/config/config.go`、`backend/internal/router/router.go`、`backend/cmd/api/main.go` から `FRONTEND_DIR` と静的 serving を除去（必要な既存 config の cleanup のみ）。

### Interfaces and environment

- `API_SERVER_ORIGIN` は server-side rewrite target、開発既定 `http://localhost:8080`。browser は `/api/:path*` のみ呼ぶ。
- `next.config.*` は Next 公式 rewrites 形式で `/api/:path*` → `${API_SERVER_ORIGIN}/api/:path*` を proxy し、OAuth callback も同じ経路にする。
- Go API は `localhost:8080`、Next は `localhost:3000`。env docs は `GoogleOAuthRedirectURL=http://localhost:3000/api/v1/auth/google/callback`、`AuthSuccessRedirectURL=http://localhost:3000/` を示す。
- Go の既存 HttpOnly SameSite=Lax cookie を変更しない。PR3 は cookie/auth flow を壊さない proxy のみ実装する。
- scripts: `dev`, `build`, `start`, `lint`, `typecheck`（必要なら package script で実行）。local Make targets は API/Next を別 process で起動する。

### TDD/acceptance steps

1. **RED:** config/rewrite contract test と placeholder smoke 条件を記述する。
2. **GREEN:** `create-next-app@latest` で scaffold、official minimum Node version を docs/package engines に固定し、lockfile を生成する。
3. `npm run lint`, `npm run typecheck`, `npm run build` を通す。
4. Go の既存 API tests を通し、`FRONTEND_DIR`/static HTML references が docs/code に残っていないことを確認する。
5. manual smoke: Next `/`、Next `/api/v1/healthz`、OAuth callback path が Go API に届くことを確認する。

## PR4: Next.js UI implementation

### Files and interfaces

Modify `frontend/app/page.tsx`, `frontend/app/layout.tsx` and add focused `components/`, `lib/api.ts`, and tests only as needed by the existing Next setup. No Go API contract changes.

- same-origin `fetch('/api/...', { credentials: 'same-origin' })` for all calls; do not expose cookie to JavaScript.
- namespace create/get, library create/list/get/PATCH visibility, version create/list/detail, verification create/detail polling UI.
- Google login is a link to `/api/v1/auth/google/login`.
- dev header auth controls are opt-in only after an explicit checkbox; display local-only `DEV_HEADER_AUTH=true` warning and never send headers by default.
- poll verification every 1 second, max 30 attempts; render source/adapter as text, not unsafe HTML.

### TDD and acceptance

1. **RED:** component/API helper tests for same-origin paths, credentials, explicit dev auth, response states, no unsafe HTML, and 30-attempt polling.
2. **GREEN:** implement UI in small components while preserving the placeholder route and API contract.
3. Run `npm run lint`, `npm run typecheck`, `npm run build`, and frontend tests.
4. Manual E2E with PostgreSQL, LocalStack, Go API, Next.js, and Worker: create namespace/private library/version, non-owner 404, publicize, verify Problem 1 with adapter, poll to `ac`, and confirm normal submission remains unchanged.
5. If DB/Worker/LocalStack is unavailable, record E2E as blocked; API/usecase tests plus Next lint/typecheck/build remain required and do not claim judge acceptance.

## Self-review checklist

- No static single-HTML frontend or `FRONTEND_DIR` serving remains in this plan/spec.
- Four PR boundaries are explicit and dependency ordered; PR3 contains setup only and PR4 contains UI only.
- Auth uses existing Go cookie/OAuth config, Next same-origin rewrites, and explicit opt-in dev headers.
- DB concurrency, transaction rollback, privacy, existing judge payload, and DB/E2E caveat have tests and acceptance criteria.
- This is a plan only: do not edit production source or commit while updating these documents.
