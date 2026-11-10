# Submission Link Pre-merge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 未マージ PR の提出リンク命名、DB 認可、Version 作成のエラーとハッシュ計算を整合させる。

**Architecture:** DB PR でテーブル・クエリ・repository を整え、積み上げた API と UI を順に追従させる。提出 URL は自己申告の文字列であり Judge に接続しない。

**Tech Stack:** Go、PostgreSQL、sqlc、Next.js、TypeScript

**Spec:** `docs/superpowers/specs/2026-11-10-submission-link-premerge-design.md`

## Global Constraints

- 既存 Judge/Submission API は変更しない。
- 非公開 Library の内容は所有者以外に返さない。
- 各 PR の責務を維持し、通常の merge で上流差分を伝播する。

## Review Focus

- Library が存在しない Version 作成は `LIBRARY_NOT_FOUND` となる。
- 呼び出し側の誤った SHA/byte size が DB CHECK 違反にならず、正しい値が保存される。
- 公開→非公開の切替と読み取りの競合でも URL・ソースを漏らさない。
- 匿名・非所有者の提出リンク詳細も SQL で非表示にする。
- UI が旧 `/verifications` を呼ばず、自己申告リンクとして表示する。

---

### Task 1: DB と repository

**Files:** `backend/db/migrations/000009_create_library_sharing.*.sql`、`backend/db/queries/library_*.sql`、`backend/internal/infra/postgres/library_sharing_repository.go`、`backend/internal/repository/library_sharing_repository.go`、`backend/internal/domain/library_sharing.go`、DB テスト・sqlc 生成物

**Interfaces:** `LibraryVersionRepository.Create` は `SourceCode` のみから digest と byte size を作る。取得メソッドは actor ID を受ける。提出リンクの DB 名と Go 型は `LibrarySubmissionLink` 系。

- [ ] 対象 Library 不在、誤った digest/size、非公開詳細・一覧を表す失敗テストを追加して RED を確認する。
- [ ] migration・クエリ・repository・domain を変更し sqlc を再生成する。
- [ ] Go テスト、DB 統合テスト、sqlc 再生成差分なし、`git diff --check` を確認する。
- [ ] DB PR にコミットする。

### Task 2: API

**Files:** `backend/internal/usecase/library_*`、`backend/internal/handler/library_sharing_handler.go`、`backend/internal/router/router.go`、`backend/docs/openapi.yaml` とテスト

**Interfaces:** `POST/GET .../versions/{version_number}/submission-links`、`GET /api/v1/library-submission-links/{id}`。JSON は `submission_url` を維持する。

- [ ] 旧 URL・レスポンス名と認可境界を検証する失敗テストを追加する。
- [ ] DB PR を merge し API/usecase/handler/OpenAPI 名を追従させる。
- [ ] Go テスト、ビルド、OpenAPI 検証を行いコミットする。

### Task 3: Next.js 基盤と UI

**Files:** PR #46 は積み上げ先のみ。PR #47 は `frontend/lib/*`、`frontend/components/*`、テストと README。

**Interfaces:** API と同じ `/submission-links` と `SubmissionLink` 型を使う。

- [ ] PR #46 に新しい API を merge し基盤差分だけであることを確認する。
- [ ] PR #47 で旧名の呼び出しを失敗テストで検出し、名称と UI を変更する。
- [ ] `npm test`、lint、typecheck、build を実行しコミットする。
- [ ] 各 PR の差分・日本語説明・ER 図を更新する。
