# 提出URLリンクへの変更 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Library Version に提出URLを登録・表示できるようにし、Library固有のコード投入・adapter・結果ポーリングをなくす。

**Architecture:** PR #44 → #45 → #46 → #47 の積み上げを維持する。DBにURLと登録者を保存し、APIで現在のLibrary公開範囲を確認する。Next.js画面はURLを送信・表示するだけで、既存Submission/Judgeの実行経路には触れない。

**Tech Stack:** PostgreSQL/sqlc、Go/chi、Next.js App Router/TypeScript。

**Spec:** [2026-11-10-verification-url-design.md](../specs/2026-11-10-verification-url-design.md)

## Global Constraints

- URLは絶対HTTP(S)、ホストあり、制御文字なし、最大2048バイト。取得・解析・判定はしない。
- 新しい登録経路はSubmission/Outbox/S3へ書き込まない。
- private Libraryは所有者以外に404とし、adminも例外にしない。
- 既存Submission APIとJudgeのコード・挙動を変更しない。
- PRごとの機能境界を維持し、依存PRの更新後に後続ブランチを追従させる。

## Review Focus

- private化とURL追加が競合した場合、非所有者のリンク登録が成功しないこと。
- 公開Libraryのリンク詳細が読めても、private化後は読めなくなること。
- URL登録でSubmission/Outbox/S3への書き込みが発生しないこと。
- スキームだけ・ホストなし・制御文字・2049バイトのURLが400になること。
- PR #47の画面がadapterと判定済みを示す文言を残さないこと。

## Task 1: PR #44 のDB差分

**Files:** `backend/db/migrations/000009_create_library_sharing.up.sql`、`backend/db/queries/library_verifications.sql`、sqlc生成物、`backend/internal/domain/library_sharing.go`、`backend/internal/repository/library_sharing_repository.go`、`backend/internal/infra/postgres/library_sharing_repository.go` と統合テスト、既存の要件・設計・計画文書。

- [ ] 失敗する統合テストを書く。URL・登録者を保存し、Submission/Outboxを作らず、privateの同時変更を拒否することを確認する。
- [ ] テストが期待どおり失敗することを確認する。
- [ ] migration・query・repositoryをURLリンク用へ変更し、sqlcを再生成する。
- [ ] DB統合テスト、`go test ./...`、`go vet ./...`、migration up/down/up、`git diff --check` を実行する。
- [ ] PR #44のブランチへコミットし、Codexレビュー後にpushする。

## Task 2: PR #45 のAPI差分

**Files:** `backend/internal/usecase/library_verification_usecase.go`、`backend/internal/handler/library_sharing_handler.go`、`backend/internal/router/router.go`、`backend/cmd/api/main.go`、`backend/docs/openapi.yaml`、関連テスト。`backend/internal/librarycombine` は削除対象。

- [ ] 失敗するAPI/usecaseテストを書く。URL登録・一覧・詳細、公開/非公開認可、URL入力エラー、Judgeへの非投入を確認する。
- [ ] テストが期待どおり失敗することを確認する。
- [ ] Usecase/handler/router/OpenAPI/API構成をURL登録へ変更し、不要な結合・storage・admission依存を削除する。
- [ ] Goテスト・vet・build、DB統合テスト、`git diff --check` を実行する。
- [ ] PR #45のブランチへコミットし、Codexレビュー後にpushする。

## Task 3: PR #46 の追従

**Files:** 原則として製品コード変更なし。積み上げ先・lockfile・Next.js buildを確認する。

- [ ] PR #45を取り込み、差分がNext.js基盤だけであることを確認する。
- [ ] `npm ci`、lint、型チェック、build、`git diff --check` を実行し、コミット・pushする。

## Task 4: PR #47 の画面差分

**Files:** `frontend/components/VerificationPanel.tsx`、`frontend/lib/api.ts`、`frontend/lib/types.ts`、`frontend/lib/verification.ts`、`frontend/lib/workspaceState.ts`、関連テスト・文書。

- [ ] 失敗するUI/APIクライアントテストを書く。VersionとURLだけで登録でき、URLを表示し、adapter・判定pollingを使わないことを確認する。
- [ ] テストが期待どおり失敗することを確認する。
- [ ] 画面・型・APIクライアントを提出リンクへ変更し、不要な状態・pollingを削除する。
- [ ] `npm test`、lint、型チェック、build、`git diff --check` を実行する。
- [ ] PR #47のブランチへコミットし、Codexレビュー後にpushする。
