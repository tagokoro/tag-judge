# Library Sharing MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ユーザー所有 Namespace の Library を公開制御つきで保存し、不変 Version と問題別 adapter を既存 Submission/Outbox/Judge で検証できるようにする。

**Architecture:** API が Library Version の Python 1 本と adapter を 1 本の `main.py` に結合し、現行の storage、`submissions`、`submission_job_outbox`、Publisher、Worker へ流す。Worker と judge-core は変更しない。静的 3 ファイルを API と同一オリジンで配信し、手動確認に使う。

**Tech Stack:** Go 1.26、chi、pgx、sqlc、PostgreSQL 16、golang-migrate、既存の S3/local storage と SQS outbox、Python 3。フロントはビルドしない HTML/CSS/JS。

**Spec:** [docs/superpowers/specs/2026-11-08-library-sharing-mvp-design.md](../specs/2026-11-08-library-sharing-mvp-design.md)

## Global Constraints

- Go module は `github.com/tagokoro/tag-judge/backend`、Go 1.26、PostgreSQL 16、Python 3 のみ。
- Library と adapter の上限はそれぞれ 100000 バイト。Version の DB check も 100000 である。
- slug 正規表現は `^[a-z0-9]+(?:-[a-z0-9]+)*$`。Namespace は 2 から 32、Library は 2 から 48。
- visibility は `public` と `private` だけ。
- 結合テンプレート、fixture の hash、outbox の JSON キーは仕様の節 7 と節 8 の値から変えない。
- エラーコードとメッセージは仕様の節 9 と節 10 の文言から変えない。
- `judge/core`、`judge/worker`、`docs/library-sharing-judge-requirements.md`、既存提出スキーマは変更しない。

## Review Focus

- library の `if __name__ == "__main__"` が解答より先に動くと、検証結果が library のデモ出力になる。期待は、fixture の stdin `1 2\n` に対する stdout `3\n` だけで `library-main` は出ない。Task 1 の Python 実行テストで固定する。
- private Library を admin が読めると、他ユーザーからのアクセス制御が破れる。期待は所有者以外の admin も `404 LIBRARY_NOT_FOUND` で、一覧にも出ない。Task 4 で固定する。
- Version 作成後の UPDATE が成功すると不変性が破れる。期待は DB が `library_versions are immutable` で拒否する。Task 2 の統合テストで固定する。
- verification 用の outbox JSON に独自フィールドがあると、既存 Publisher がジョブを復元できない。期待は仕様節 8 の 5 キーだけを持つ。Task 6 で固定する。
- public Library を読める他ユーザーが他人の adapter を読めると、問題別の解法が漏れる。期待は `403 LIBRARY_VERIFICATION_FORBIDDEN` で、レスポンスに adapter 本文が無い。Task 6 で固定する。

## File Structure

作成:

- `backend/internal/librarycombine/combine.go`: ソース検証、SHA-256、結合。HTTP と DB を知らない。
- `backend/internal/librarycombine/combine_test.go`: fixture のバイト一致と `python3` 実行。
- `backend/db/migrations/000007_create_library_sharing.up.sql`: 仕様節 6.1。
- `backend/db/migrations/000007_create_library_sharing.down.sql`: 仕様節 6.2。
- `backend/db/queries/namespaces.sql`: Namespace の sqlc クエリ。
- `backend/db/queries/libraries.sql`: Library の sqlc クエリ。
- `backend/db/queries/library_versions.sql`: Version 採番 insert と取得。
- `backend/db/queries/library_verifications.sql`: verification の insert と join 取得。
- `backend/internal/domain/library_sharing.go`: visibility、slug、4 つのドメイン型。
- `backend/internal/repository/library_sharing_repository.go`: 4 つの repository interface と params。
- `backend/internal/usecase/namespace_usecase.go`: Namespace 作成と取得。
- `backend/internal/usecase/library_usecase.go`: Library の作成、更新、認可つき取得。
- `backend/internal/usecase/library_version_usecase.go`: 不変 Version の作成と取得。
- `backend/internal/usecase/library_verification_usecase.go`: 結合、storage、既存提出条件、検証開始と取得。
- `backend/internal/usecase/library_sharing_usecase_test.go`: 上記 usecase の fake テスト。
- `backend/internal/handler/library_sharing_handler.go`: HTTP decode とレスポンス。既存の `decodeJSON` と `parseOptionalInt` を使う。
- `backend/internal/handler/library_sharing_handler_test.go`: source / adapter の非公開、一覧 shape、ID とページングの HTTP 契約。
- `backend/internal/infra/postgres/library_sharing_repository.go`: sqlc と、verification 用の既存 outbox insert。
- `backend/internal/infra/postgres/library_sharing_integration_test.go`: trigger、採番競合、payload、transaction rollback、秘匿、verify 時の認可再確認。
- `frontend/index.html`、`frontend/app.js`、`frontend/styles.css`: 手動確認画面。

変更:

- `backend/internal/router/router.go`: 新ルートと、3 パスだけの静的配信。
- `backend/internal/router/router_test.go`: 新ハンドラを fake で組む。既存 `newTestRouter` は新しい必須フィールドをゼロ値のままにしない。
- `backend/cmd/api/main.go`: repository、usecase、handler の組み立て。
- `backend/internal/config/config.go`: `FrontendDir`。
- `backend/docs/openapi.yaml`: 節 10 のパスだけ追加する。
- `Makefile`: `run-api` が `FRONTEND_DIR` を渡す。
- `.env.example`: `FRONTEND_DIR` のコメントを 1 行足す。
- `README.md`: 手動確認への短い導線だけ足す。機能説明の書き換えはしない。

変更しない:

- `judge/core/**`、`judge/worker/**`、`docs/library-sharing-judge-requirements.md`。
- `backend/db/queries/submissions.sql`、`submissionOutboxPayload` のフィールド、既存提出 usecase の挙動。

---

### Task 1: Library と adapter の結合

**Files:**

- Create: `backend/internal/librarycombine/combine.go`
- Test: `backend/internal/librarycombine/combine_test.go`

**Interfaces:**

- Consumes: なし。
- Produces: `librarycombine.ErrEmptySource`、`ErrNULSource`、`ErrSourceTooLarge`、`MaxSourceBytes = 100000`、`ContentSHA256(source string) string`、`ValidateSource(source string) error`、`Combine(librarySource, adapterSource string) (string, error)`。

- [ ] **Step 1: Write the failing test**

`combine_test.go` に次を置く。fixture の文字列は仕様節 7.3 とバイト単位で同じにする。

```go
func TestCombineRunsAdapterWithoutLibraryMain(t *testing.T) {
    library := "def add(a, b):\n    return a + b\n\nif __name__ == \"__main__\":\n    print(\"library-main\")\n"
    adapter := "a, b = map(int, input().split())\nprint(add(a, b))\n"
    if ContentSHA256(library) != "b24916acf9a8670a4e088af2bbc2591d02eca1f33ddad1aec0ed3a41d57568af" {
        t.Fatalf("library hash = %s", ContentSHA256(library))
    }
    if ContentSHA256(adapter) != "b8d685a728f43ee0c53f5f299234c42d5ebbe0f423bb5b18d16a7ecaf2c03f21" {
        t.Fatalf("adapter hash = %s", ContentSHA256(adapter))
    }
    got, err := Combine(library, adapter)
    if err != nil {
        t.Fatalf("Combine: %v", err)
    }
    want := "import base64\n_library = base64.b64decode('ZGVmIGFkZChhLCBiKToKICAgIHJldHVybiBhICsgYgoKaWYgX19uYW1lX18gPT0gIl9fbWFpbl9fIjoKICAgIHByaW50KCJsaWJyYXJ5LW1haW4iKQo=').decode(\"utf-8\")\n_adapter = base64.b64decode('YSwgYiA9IG1hcChpbnQsIGlucHV0KCkuc3BsaXQoKSkKcHJpbnQoYWRkKGEsIGIpKQo=').decode(\"utf-8\")\n_namespace = {\"__name__\": \"library\"}\nexec(compile(_library, \"<library>\", \"exec\"), _namespace)\n_namespace[\"__name__\"] = \"__main__\"\nexec(compile(_adapter, \"<adapter>\", \"exec\"), _namespace)\n"
    if got != want {
        t.Fatalf("combined source mismatch:\n%s", got)
    }
    cmd := exec.Command("python3", "-c", got)
    cmd.Stdin = strings.NewReader("1 2\n")
    out, err := cmd.CombinedOutput()
    if err != nil || string(out) != "3\n" {
        t.Fatalf("python output = %q, err = %v", out, err)
    }
}

func TestValidateSourceBoundaries(t *testing.T) {
    if !errors.Is(ValidateSource(" \n"), ErrEmptySource) {
        t.Fatal("blank source must be empty")
    }
    if !errors.Is(ValidateSource("a\x00b"), ErrNULSource) {
        t.Fatal("NUL must be rejected")
    }
    if !errors.Is(ValidateSource(strings.Repeat("a", MaxSourceBytes+1)), ErrSourceTooLarge) {
        t.Fatal("100001 bytes must be rejected")
    }
    if err := ValidateSource(strings.Repeat("a", MaxSourceBytes)); err != nil {
        t.Fatalf("100000 bytes must pass: %v", err)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/librarycombine -count=1`

Expected: FAIL。`librarycombine` が存在しない、または `Combine` が未定義。

- [ ] **Step 3: Implement the combiner**

`combine.go` に仕様節 7 の検証とテンプレートを実装する。`Combine` は両方を `ValidateSource` してから、base64 の `StdEncoding` で埋めたテンプレートを返す。hash は保存文字列の生バイトで計算し、trim しない。

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/librarycombine -count=1`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/librarycombine/combine.go backend/internal/librarycombine/combine_test.go
git commit -m "test: combine library version with problem adapter"
```

### Task 2: Namespace、Library、Version、verification の schema

**Files:**

- Create: `backend/db/migrations/000007_create_library_sharing.up.sql`
- Create: `backend/db/migrations/000007_create_library_sharing.down.sql`
- Test: `backend/internal/infra/postgres/library_sharing_integration_test.go`

**Interfaces:**

- Consumes: 既存の `users`、`problems`、`submissions`。
- Produces: 仕様節 6 のテーブル、制約名、trigger 名 `library_versions_no_update_delete`、関数 `reject_library_version_mutation`。

- [ ] **Step 1: Write the failing integration test**

`TestLibraryVersionIsImmutable` を追加する。`SUBMISSION_DETAILS_TEST_DATABASE_URL` が空なら skip する。DB 準備は `submission_details_integration_test.go` の isolated schema と `../../../db/migrations/*.up.sql` 適用に合わせ、ヘルパーを共有できるならそれを使う。共有で既存テストの cleanup が変わるなら、同じ手順をこのファイルに複製する。

テストは seed author と同じ UUID の user を挿入し、namespace、private library、version を SQL で作る。その後 `UPDATE library_versions SET source_code = source_code || '# mutated' WHERE version_number = 1` を実行し、error 文字列に `library_versions are immutable` が含まれることを確認する。さらに、誤った `byte_size` または `content_sha256` の Version と、本文に一致しない `adapter_sha256` の verification を SQL で挿入し、それぞれ DB 制約で拒否されることを確認する。採番は Task 5 の repository 統合テストで扱い、この schema test で明示値 `2` を挿入して採番をテストしたことにはしない。

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && SUBMISSION_DETAILS_TEST_DATABASE_URL='postgres://judge_user:judge_pass@localhost:5432/judge?sslmode=disable' go test ./internal/infra/postgres -run TestLibraryVersionIsImmutable -count=1`

Expected: FAIL。`namespaces` が無い、または trigger が無く UPDATE が成功する。ローカル DB が無い場合は CI と同じ PostgreSQL 16 を起動してから実行する。skip ではこの step を完了にしない。

- [ ] **Step 3: Add migration 000007**

up は仕様節 6.1、down は節 6.2 をそのまま書く。`library_verifications` に `problem_id` / `user_id` を重複保存せず、既存 `submissions` を正とする。既存 migration は編集しない。

- [ ] **Step 4: Run test to verify it passes**

Run: 同じ `TestLibraryVersionIsImmutable`。続けて `make migrate-up && make migrate-down && make migrate-up`。

Expected: テスト PASS。down のあと `namespaces` が無く、再 up で戻る。

- [ ] **Step 5: Commit**

```bash
git add backend/db/migrations/000007_create_library_sharing.up.sql backend/db/migrations/000007_create_library_sharing.down.sql backend/internal/infra/postgres/library_sharing_integration_test.go
git commit -m "feat: add immutable library sharing schema"
```

### Task 3: Namespace API

**Files:**

- Create: `backend/internal/domain/library_sharing.go`
- Create: `backend/internal/repository/library_sharing_repository.go`
- Create: `backend/db/queries/namespaces.sql`
- Create: `backend/internal/usecase/namespace_usecase.go`
- Create: `backend/internal/usecase/library_sharing_usecase_test.go`
- Create: `backend/internal/handler/library_sharing_handler.go`
- Create: `backend/internal/infra/postgres/library_sharing_repository.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/docs/openapi.yaml`
- Generated: `backend/internal/infra/postgres/sqlc/`

**Interfaces:**

- Consumes: `auth.Actor`、`validateActor`、`apperror`。
- Produces:
  - `domain.Namespace{ID, OwnerUserID, Slug, DisplayName string; CreatedAt, UpdatedAt time.Time}`
  - `domain.ValidNamespaceSlug(slug string) bool`
  - `repository.NamespaceRepository` の `Create(ctx, CreateNamespaceParams) (*domain.Namespace, error)`、`FindByOwner(ctx, ownerUserID string)`、`FindBySlug(ctx, slug string)`
  - `usecase.NewNamespaceUsecase(namespaces repository.NamespaceRepository) *NamespaceUsecase`
  - `CreateNamespace(ctx, actor, slug, displayName string) (*domain.Namespace, error)`
  - `GetMine(ctx, actor) (*domain.Namespace, error)`
  - `GetBySlug(ctx, slug string) (*domain.Namespace, error)`
  - `handler.NewNamespaceHandler(namespaces *usecase.NamespaceUsecase) *NamespaceHandler`
  - router の `Handlers.Namespaces *handler.NamespaceHandler`

- [ ] **Step 1: Write the failing usecase tests**

`TestCreateNamespace` は actor `00000000-0000-4000-8000-000000000001`、slug `seed-author`、display name ` Seed Author ` を渡し、repository へ渡る display name が `Seed Author` であることを確認する。

`TestCreateNamespaceRejectsSecondNamespace` は repository が制約名 `namespaces_owner_user_unique` の一意制約エラーを返したとき、`errors.Is(err, apperror.ErrConflict)` かつ code `NAMESPACE_ALREADY_EXISTS`、message `user already has a namespace` であることを確認する。slug 制約 `namespaces_slug_unique` は code `NAMESPACE_SLUG_TAKEN`、message `namespace slug is already taken`。

`TestCreateNamespaceRejectsSlug` は `A`、`-ab`、`a--b`、`a`、33 文字を `INVALID_SLUG` / `slug must be 2-32 lowercase letters, digits, or single hyphens` で拒否し、repository を呼ばない。

同じ task で `TestNamespaceUniqueConstraints` も先に書く。環境変数が空なら skip する。repository の `Create` を 2 回呼び、同一 user は `NAMESPACE_ALREADY_EXISTS`、別 user の同一 slug は `NAMESPACE_SLUG_TAKEN` であることを確認する。このテストは Step 2 では未実装により FAIL し、Step 4 で PASS する。

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/usecase -run 'TestCreateNamespace|TestCreateNamespaceRejects' -count=1`

Expected: FAIL。型または関数が未定義。

DB を起動した状態で続けて実行する。

Run: `cd backend && SUBMISSION_DETAILS_TEST_DATABASE_URL='postgres://judge_user:judge_pass@localhost:5432/judge?sslmode=disable' go test ./internal/infra/postgres -run TestNamespaceUniqueConstraints -count=1`

Expected: FAIL。repository または query が未実装。skip では RED 完了としない。

- [ ] **Step 3: Implement Namespace usecase and map constraint names**

usecase は形式を検証してから `NamespaceRepository.Create` を呼ぶ。display name の文字数は既存 Group と同じ `len([]rune(value))` で数え、DB の `char_length` と合わせる。fake は変換済みの `apperror.Conflict` を返し、usecase テストはそれを通すことだけを確認する。PostgreSQL 実装は `pgconn.PgError.ConstraintName` を見て、`namespaces_owner_user_unique` を `NAMESPACE_ALREADY_EXISTS`、`namespaces_slug_unique` を `NAMESPACE_SLUG_TAKEN` に変換する。この変換はエクスポート関数に切り出さない。同じ task の統合テスト `TestNamespaceUniqueConstraints` が、同じ user の 2 件目と slug 衝突を repository 経由で作り、上記 code と message を確認する。

sqlc クエリ名は `CreateNamespace`、`GetNamespaceByOwner`、`GetNamespaceBySlug`。UUID は `::text` で返し、引数は既存クエリと同じ `((sqlc.arg(owner_user_id)::text)::uuid)` を使う。

ルート:

- `POST /api/v1/namespaces` は `RequireAuth`
- `GET /api/v1/me/namespace` は `RequireAuth`
- `GET /api/v1/namespaces/{namespace_slug}` は認証不要

`main.go` で `postgres.NewNamespaceRepository` と handler を既存の組み立て方で接続する。`router_test.go` の `newTestRouter` は非 nil の `NamespaceHandler` を渡す。nil のまま route を登録しない。OpenAPI にはこの 3 パスと節 10.1 の schema だけを足す。

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && sqlc generate && go test ./internal/usecase ./internal/router -count=1`

Run: `cd backend && SUBMISSION_DETAILS_TEST_DATABASE_URL='postgres://judge_user:judge_pass@localhost:5432/judge?sslmode=disable' go test ./internal/infra/postgres -run TestNamespaceUniqueConstraints -count=1`

Expected: 両方 PASS。GREEN の統合テストを skip で代替しない。コンパイルエラーは不可。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/domain/library_sharing.go backend/internal/repository/library_sharing_repository.go backend/db/queries/namespaces.sql backend/internal/usecase/namespace_usecase.go backend/internal/usecase/library_sharing_usecase_test.go backend/internal/handler/library_sharing_handler.go backend/internal/infra/postgres/library_sharing_repository.go backend/internal/infra/postgres/sqlc backend/internal/router/router.go backend/internal/router/router_test.go backend/cmd/api/main.go backend/docs/openapi.yaml
git commit -m "feat: add user-owned namespace API"
```

### Task 4: Library の公開制御

**Files:**

- Modify: `backend/internal/domain/library_sharing.go`
- Modify: `backend/internal/repository/library_sharing_repository.go`
- Create: `backend/db/queries/libraries.sql`
- Create: `backend/internal/usecase/library_usecase.go`
- Modify: `backend/internal/usecase/library_sharing_usecase_test.go`
- Modify: `backend/internal/handler/library_sharing_handler.go`
- Create: `backend/internal/handler/library_sharing_handler_test.go`
- Modify: `backend/internal/infra/postgres/library_sharing_repository.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/docs/openapi.yaml`
- Generated: `backend/internal/infra/postgres/sqlc/`

**Interfaces:**

- Consumes: Task 3 の `NamespaceRepository`。
- Produces:
  - `domain.LibraryVisibility` の `public` と `private`、`Valid() bool`
  - `domain.Library{ID, NamespaceID, NamespaceSlug, Slug, Name, Description string; Visibility LibraryVisibility; CreatedAt, UpdatedAt time.Time}`
  - `repository.LibraryRepository` の `Create`、`Update`、`FindBySlug(ctx, namespaceID, slug string)`、`ListPublic(ctx, namespaceID string, limit, offset int)`、`ListAll(ctx, namespaceID string, limit, offset int)`
  - `usecase.NewLibraryUsecase(namespaces NamespaceRepository, libraries LibraryRepository) *LibraryUsecase`
  - `Create`、`Update`、`Get`、`List`
  - `AuthorizeOwner(ctx, actor, namespaceSlug, librarySlug string) (*domain.Library, error)`。所有者なら Library を返し、public の非所有者は `LIBRARY_FORBIDDEN`、private または不存在は `LIBRARY_NOT_FOUND` を返す。
  - `handler.NewLibraryHandler(libraries *usecase.LibraryUsecase) *LibraryHandler`
  - router の `Handlers.Libraries *handler.LibraryHandler`

- [ ] **Step 1: Write the failing authorization tests**

`TestGetPrivateLibraryHidesFromAdmin` は、所有者ではない admin actor に private library の `Get` をさせ、`apperror.ErrNotFound`、code `LIBRARY_NOT_FOUND`、message `library not found` を確認する。fake の `ListPublic` が呼ばれ、`ListAll` は呼ばれないことも `List` で確認する。

`TestUpdatePublicLibraryRequiresOwner` は、public library を他ユーザーが PATCH すると `403 LIBRARY_FORBIDDEN` / `library owner required` になり、repository の `Update` は呼ばれないことを確認する。所有者の PATCH は `visibility=public` を保存する。

`TestCreateLibraryValidatesContract` は slug `a--b`、name が空白、description が 2001 文字、visibility `group` をそれぞれ `INVALID_SLUG`、`INVALID_NAME`、`INVALID_DESCRIPTION`、`INVALID_VISIBILITY` と仕様のメッセージで拒否する。空の PATCH は `EMPTY_UPDATE` / `at least one field is required`。

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/usecase -run 'TestGetPrivateLibraryHidesFromAdmin|TestUpdatePublicLibraryRequiresOwner|TestCreateLibraryValidatesContract' -count=1`

Expected: FAIL。`LibraryUsecase` が未定義。

- [ ] **Step 3: Implement Library authorization**

読み取りは `FindBySlug` のあと、private かつ非所有者なら NotFound を返す。一覧は所有者だけ `ListAll`、それ以外は `ListPublic`。作成と更新は所有者だけ。public を読める非所有者の更新は Forbidden、private の更新は NotFound。name と description の文字数は `len([]rune(value))` で数え、DB の `char_length` と合わせる。slug 衝突は制約名 `libraries_namespace_slug_unique` を `LIBRARY_SLUG_TAKEN` に変換する。

sqlc は `CreateLibrary`、`UpdateLibrary`、`GetLibraryBySlug`、`ListPublicLibraries`、`ListNamespaceLibraries`。一覧順は `created_at DESC, slug ASC`。

ルートは仕様節 10.2。すべて Namespace slug から所有者を引く。`POST` と `PATCH` は `RequireAuth`。`GET` 一覧と詳細は未認証でも呼べる。`main.go` と OpenAPI を同じ契約で更新する。`newTestRouter` は非 nil の `LibraryHandler` を渡す。router テストは dev header の admin `00000000-0000-4000-8000-000000000002` で private GET が 404 になることを 1 本足す。

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && sqlc generate && go test ./internal/usecase ./internal/router -count=1`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/domain/library_sharing.go backend/internal/repository/library_sharing_repository.go backend/db/queries/libraries.sql backend/internal/usecase/library_usecase.go backend/internal/usecase/library_sharing_usecase_test.go backend/internal/handler/library_sharing_handler.go backend/internal/handler/library_sharing_handler_test.go backend/internal/infra/postgres/library_sharing_repository.go backend/internal/infra/postgres/sqlc backend/internal/router/router.go backend/internal/router/router_test.go backend/cmd/api/main.go backend/docs/openapi.yaml
git commit -m "feat: control library visibility for other users"
```

### Task 5: 不変な Library Version

**Files:**

- Modify: `backend/internal/domain/library_sharing.go`
- Modify: `backend/internal/repository/library_sharing_repository.go`
- Create: `backend/db/queries/library_versions.sql`
- Create: `backend/internal/usecase/library_version_usecase.go`
- Modify: `backend/internal/usecase/library_sharing_usecase_test.go`
- Modify: `backend/internal/handler/library_sharing_handler.go`
- Modify: `backend/internal/handler/library_sharing_handler_test.go`
- Modify: `backend/internal/infra/postgres/library_sharing_repository.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/docs/openapi.yaml`
- Generated: `backend/internal/infra/postgres/sqlc/`

**Interfaces:**

- Consumes: `librarycombine.ValidateSource`、`ContentSHA256`、Task 4 の `LibraryUsecase.AuthorizeOwner` と `Get`。
- Produces:
  - `domain.LibraryVersion{ID, LibraryID, SourceCode, ContentSHA256, CreatedBy string; VersionNumber, ByteSize int; CreatedAt time.Time}`
  - `repository.LibraryVersionRepository` の `Create(ctx, CreateLibraryVersionParams) (*domain.LibraryVersion, error)`、`FindByNumber(ctx, libraryID string, versionNumber int)`、`ListSummaries(ctx, ListLibraryVersionsParams{LibraryID, Limit, Offset}) ([]domain.LibraryVersion, error)`
  - `usecase.NewLibraryVersionUsecase(libraries *LibraryUsecase, versions LibraryVersionRepository) *LibraryVersionUsecase`
  - `Create`、`Get`、`List`。更新メソッドは作らない。
  - `handler.NewLibraryVersionHandler(versions *usecase.LibraryVersionUsecase) *LibraryVersionHandler`
  - router の `Handlers.LibraryVersions *handler.LibraryVersionHandler`

- [ ] **Step 1: Write the failing tests**

`TestCreateLibraryVersionStoresExactHash` は仕様の add fixture を所有者として作成し、repository が受け取る `ContentSHA256` が `b24916acf9a8670a4e088af2bbc2591d02eca1f33ddad1aec0ed3a41d57568af`、`ByteSize` が 86、`SourceCode` が入力と完全一致であることを確認する。末尾改行を削らない。

`TestCreateLibraryVersionRejectsMutationByNonOwner` は public library への他ユーザー作成が `LIBRARY_FORBIDDEN`、private library への admin 作成が `LIBRARY_NOT_FOUND` であることを確認する。

`TestListLibraryVersionsOmitsSource` は handler テストとして、一覧 JSON に `source_code` キーが無く、詳細 JSON には fixture 本文があることを確認する。

`TestListLibraryVersionsValidatesPagination` は limit 省略時に 20、`limit=0` / `101` に `INVALID_LIMIT`、負の offset に `INVALID_OFFSET` を返し、repository へ limit / offset が渡ることを確認する。

PostgreSQL 統合テスト `TestCreateLibraryVersionAllocatesConsecutiveNumbersConcurrently` は同じ Library に対して repository の `Create` を 2 goroutine から同時に開始し、両方が完了して返る番号が重複のない `1, 2` になることを確認する。タイムアウトを設け、直列の通常作成だけで競合安全性を確認したことにしない。

router テストで `PATCH` と `DELETE` の `.../versions/1` が `405` であることを確認する。

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/usecase ./internal/handler ./internal/router -run 'TestCreateLibraryVersion|TestListLibraryVersions|TestLibraryVersionMethodNotAllowed' -count=1`

Expected: FAIL。Version API が未定義、または PATCH が 405 でない。

DB を起動した状態で続けて実行する。

Run: `cd backend && SUBMISSION_DETAILS_TEST_DATABASE_URL='postgres://judge_user:judge_pass@localhost:5432/judge?sslmode=disable' go test ./internal/infra/postgres -run TestCreateLibraryVersionAllocatesConsecutiveNumbersConcurrently -count=1`

Expected: FAIL。repository / query が未実装。skip では RED 完了としない。

- [ ] **Step 3: Implement immutable version creation**

`CreateLibraryVersion` は仕様節 6.3 の `FOR UPDATE` クエリをそのまま使う。採番を Go 側で `count + 1` しない。一覧クエリは `source_code` を SELECT せず、`LIMIT` / `OFFSET` を受ける。詳細と作成だけが本文を返す。handler の一覧レスポンス型にも `source_code` フィールドを置かない。

`Create` は `AuthorizeOwner` を通った Library だけに Version を作る。`Get` と `List` は `LibraryUsecase.Get` で読み取り可否を判定する。`List` は Submission 一覧と同じ既定 20、最大 100、offset 0 以上を検証する。採番を Go 側で計算する処理は足さない。OpenAPI に POST、ページングする GET 一覧、GET 詳細だけを追加する。`newTestRouter` は非 nil の `LibraryVersionHandler` を渡す。

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && sqlc generate && go test ./internal/usecase ./internal/handler ./internal/router ./internal/librarycombine -count=1`

Run: `cd backend && SUBMISSION_DETAILS_TEST_DATABASE_URL='postgres://judge_user:judge_pass@localhost:5432/judge?sslmode=disable' go test ./internal/infra/postgres -run TestCreateLibraryVersionAllocatesConsecutiveNumbersConcurrently -count=1`

Expected: 両方 PASS。GREEN の同時採番テストを skip で代替しない。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/domain/library_sharing.go backend/internal/repository/library_sharing_repository.go backend/db/queries/library_versions.sql backend/internal/usecase/library_version_usecase.go backend/internal/usecase/library_sharing_usecase_test.go backend/internal/handler/library_sharing_handler.go backend/internal/handler/library_sharing_handler_test.go backend/internal/infra/postgres/library_sharing_repository.go backend/internal/infra/postgres/sqlc backend/internal/router/router.go backend/internal/router/router_test.go backend/cmd/api/main.go backend/docs/openapi.yaml
git commit -m "feat: add immutable library versions"
```

### Task 6: 既存 Submission/Outbox/Judge への verify

**Files:**

- Modify: `backend/internal/domain/library_sharing.go`
- Modify: `backend/internal/repository/library_sharing_repository.go`
- Create: `backend/db/queries/library_verifications.sql`
- Create: `backend/internal/usecase/library_verification_usecase.go`
- Modify: `backend/internal/usecase/library_sharing_usecase_test.go`
- Modify: `backend/internal/handler/library_sharing_handler.go`
- Modify: `backend/internal/handler/library_sharing_handler_test.go`
- Modify: `backend/internal/infra/postgres/library_sharing_repository.go`
- Modify: `backend/internal/infra/postgres/library_sharing_integration_test.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/docs/openapi.yaml`
- Generated: `backend/internal/infra/postgres/sqlc/`

**Interfaces:**

- Consumes: `librarycombine.Combine`、`repository.PublicProblemReader`、`repository.SourceCodeStorage`、`repository.SubmissionRepository.FindByID` と `ListCaseResults`、既存の `submissionOutboxPayload`。
- Produces:
  - `domain.LibraryVerification`。仕様節 10.4 の識別子、status、score、adapter、judged_at を持つ。
  - `repository.LibraryVerificationRepository.CreateWithSubmission(ctx, CreateLibraryVerificationParams) (*domain.LibraryVerification, *domain.Submission, error)`。params は `ActorUserID`、`LibraryID`、`LibraryVersionID` と Submission / adapter の値を持ち、DB transaction 内で Library の可視性と Version 所属を再確認する。`ProblemID` と開始者は新規 verification 行へ重複保存しない。
  - `FindByID(ctx, id int64) (*domain.LibraryVerification, error)`
  - `ListByUserID(ctx, userID string, limit, offset int) ([]domain.LibraryVerification, error)`
  - `usecase.NewLibraryVerificationUsecase(versions *LibraryVersionUsecase, problems repository.PublicProblemReader, storage repository.SourceCodeStorage, verifications LibraryVerificationRepository, submissions repository.SubmissionRepository) *LibraryVerificationUsecase`
  - `Create(ctx, actor, namespaceSlug, librarySlug string, versionNumber int, problemID int64, adapterSource string) (*domain.LibraryVerification, error)`
  - `Get(ctx, actor, id int64) (*domain.LibraryVerification, []domain.SubmissionCaseResult, error)`
  - `List(ctx, actor, limit int, limitProvided bool, offset int) ([]domain.LibraryVerification, error)`
  - `handler.NewLibraryVerificationHandler(verifications *usecase.LibraryVerificationUsecase) *LibraryVerificationHandler`
  - router の `Handlers.LibraryVerifications *handler.LibraryVerificationHandler`

- [ ] **Step 1: Write the failing tests**

`TestVerifyPublicLibraryUsesExistingSubmissionPath` は、他ユーザーが public library の version 1 と problem 42 を検証したとき、storage に渡る本文が `Combine` の結果と一致し、repository が受け取る `TestCasesKey` が problem の `current_test_cases_key`、`TimeLimitMS` が problem の値、language が `python`、status が `pending` であることを確認する。ソースキーは `^submissions/.+/problems/42/[0-9a-f]{32}\.py$` に一致する。

`TestVerifyPrivateLibraryIsHiddenFromAdmin` は admin の private verify が `LIBRARY_NOT_FOUND` で、storage も repository も呼ばれないことを確認する。

`TestVerifyRejectsProblemWithoutCases` は空の `CurrentTestCasesKey` で `409 PROBLEM_HAS_NO_TEST_CASES` / `problem has no test cases` を確認する。fake が `FindPublicByID` で NotFound を返す private problem は storage を呼ばない。

`TestVerifyRejectsInvalidAdapter` は空白、NUL、100001 byte の adapter を `INVALID_ADAPTER_SOURCE` / `adapter_source must be 1-100000 bytes, non-blank, and must not contain NUL` で拒否し、storage と repository を呼ばないことを確認する。

`TestVerifyValidatesIdentifiers` は `problem_id <= 0`、`version_number <= 0` を repository / storage 呼び出し前に仕様の code と message で拒否することを確認する。handler テストは version が非数値または Go の `int` 範囲外、verification ID が非数値または 0 以下の場合の HTTP 契約を固定する。

`TestGetVerificationHidesAdapterFromOtherUser` は、public library を読める他ユーザーの `Get` が `403 LIBRARY_VERIFICATION_FORBIDDEN` / `library verification is not accessible` であることを確認する。handler テストでは、その 403 body に adapter fixture が含まれないことを確認する。開始者の詳細は adapter を含み、`pending` の `case_results` は `[]` である。

統合テスト `TestLibraryVerificationOutboxPayloadMatchesSubmissionJob` は、実際に `CreateWithSubmission` したあと `submission_job_outbox.payload` のキー集合が `submissionId`、`language`、`sourceCodeKey`、`testCasesKey`、`timeLimitMs` だけであり、`language` が `python` であることを確認する。

統合テスト `TestLibraryVerificationRollsBackSubmissionAndOutbox` は、存在しない Version ID など verification insert だけが失敗する入力を与え、呼び出し前後で `submissions`、`submission_job_outbox`、`library_verifications` の件数がすべて変わらないことを確認する。

統合テスト `TestLibraryVerificationRechecksVisibilityInTransaction` は、非所有者用 params を作ったあと Library を private に変更してから `CreateWithSubmission` を呼び、`LIBRARY_NOT_FOUND` になり 3 テーブルへ行が残らないことを確認する。repository は transaction 内で Namespace を join した Library 行を `FOR SHARE` し、所有者または public の条件と Version の所属を同じ snapshot で確認する。

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/usecase ./internal/handler -run 'TestVerify|TestGetVerificationHidesAdapterFromOtherUser|TestLibraryVerificationHandler' -count=1`

Expected: FAIL。verification usecase が未定義。

DB を起動した状態で続けて実行する。

Run: `cd backend && SUBMISSION_DETAILS_TEST_DATABASE_URL='postgres://judge_user:judge_pass@localhost:5432/judge?sslmode=disable' go test ./internal/infra/postgres -run 'TestLibraryVerification(OutboxPayloadMatchesSubmissionJob|RollsBackSubmissionAndOutbox|RechecksVisibilityInTransaction)' -count=1`

Expected: FAIL。repository / transaction が未実装。skip では RED 完了としない。

- [ ] **Step 3: Implement verification without changing the judge**

処理順は、library の読み取り認可、version の所属確認、`FindPublicByID`、空キー拒否、adapter の `ValidateSource` と `INVALID_ADAPTER_SOURCE` への変換、`Combine`、storage `PutSourceCode`、同一 DB トランザクションでの Library 可視性 / Version 所属の再確認、submission insert、既存 `submissionOutboxPayload` の marshal、outbox insert、verification insert である。outbox SQL は `submission_repository.go` の insert と同じ文面にする。payload 用の新しい struct は作らない。

transaction 内の再確認は Library と Namespace を join し、Namespace owner が actor と一致するか Library が public の場合だけ進める。Library 行は `FOR SHARE` でロックするため、同時の visibility PATCH と commit 順で直列化される。再確認失敗は `LIBRARY_NOT_FOUND` とし、submission / outbox / verification を一切 commit しない。`library_verifications.problem_id` / `user_id` は存在しないため、詳細と一覧は `submissions` join から problem と開始者を取得する。

`SubmissionUsecase.Create` は呼ばない。呼ぶと verification 行と同じトランザクションにならない。Worker、core、`submissions.sql` と既存 Submission schema は編集しない。

作成レスポンスには adapter 本文を出さない。詳細は開始者または `actor.Role.CanViewAnySubmission()` の admin だけに本文とケース結果を返す。一覧は呼び出しユーザー自身だけで、adapter とケース結果を省く。status は join した `submissions.status` を使う。

OpenAPI に POST verification、GET 詳細、GET 一覧を追加する。`main.go` は既存の `sourceCodeStorage` と `problemRepo` を渡す。新しい dispatcher は作らない。

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && sqlc generate && go test ./internal/usecase ./internal/handler ./internal/router ./internal/librarycombine -count=1`

Run: `cd backend && SUBMISSION_DETAILS_TEST_DATABASE_URL='postgres://judge_user:judge_pass@localhost:5432/judge?sslmode=disable' go test ./internal/infra/postgres -run 'TestLibraryVerification(OutboxPayloadMatchesSubmissionJob|RollsBackSubmissionAndOutbox|RechecksVisibilityInTransaction)' -count=1`

Expected: 両方 PASS。GREEN の transaction / payload テストを skip で代替しない。

続けて変更がないことを確認する。

Run: `git diff -- judge/core judge/worker && cargo test --manifest-path judge/core/Cargo.toml && cargo test --manifest-path judge/worker/Cargo.toml`

Expected: diff は空。既存 Rust テストは PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/domain/library_sharing.go backend/internal/repository/library_sharing_repository.go backend/db/queries/library_verifications.sql backend/internal/usecase/library_verification_usecase.go backend/internal/usecase/library_sharing_usecase_test.go backend/internal/handler/library_sharing_handler.go backend/internal/handler/library_sharing_handler_test.go backend/internal/infra/postgres/library_sharing_repository.go backend/internal/infra/postgres/library_sharing_integration_test.go backend/internal/infra/postgres/sqlc backend/internal/router/router.go backend/internal/router/router_test.go backend/cmd/api/main.go backend/docs/openapi.yaml
git commit -m "feat: verify library versions through existing submissions"
```

### Task 7: 同一オリジンの最小フロントと手動確認

**Files:**

- Create: `frontend/index.html`
- Create: `frontend/app.js`
- Create: `frontend/styles.css`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `Makefile`
- Modify: `.env.example`
- Modify: `README.md`

**Interfaces:**

- Consumes: 節 10 の API と節 11.1 の要素 id。
- Produces: `config.Config.FrontendDir`。環境変数 `FRONTEND_DIR`、既定は空文字。`router.Handlers.FrontendDir string`。空またはディレクトリ不存在なら静的ルートを登録しない。

- [ ] **Step 1: Write the failing static contract test**

`TestFrontendDoesNotShadowAPI` は、一時ディレクトリに仕様の id と API パスを含む 3 ファイルを置き、`FrontendDir` をそのディレクトリにして router を作る。`GET /` は 200 で `create-version` と `switch-user` を含み、`GET /app.js` は Namespace 作成、Library 作成 / GET / PATCH、Version 作成、verification 作成 / GET の API パスを含む。`GET /api/v1/problems` は従来どおり 200 で、body が HTML ではない。`FrontendDir` が空の router では `GET /` が 404 である。

`app.js` の契約テストでは、API から受け取った `source_code` と `adapter_source` を表示する箇所が `textContent` を使い、`innerHTML` を含まないことを確認する。

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/router -run TestFrontendDoesNotShadowAPI -count=1`

Expected: FAIL。`FrontendDir` が無い、または `GET /` が 404。

- [ ] **Step 3: Implement the three static files and explicit routes**

`config.Load` は `FRONTEND_DIR` を読む。router は `/`、`/app.js`、`/styles.css` だけを `http.ServeFile` で返す。catch-all は登録しない。ディレクトリが無ければ warning を出して API 起動を続ける。

画面は仕様節 11.1 の id、初期値、cookie 優先、dev header の明示、1 秒間隔で最大 30 回のポーリングを実装する。visibility 変更は `PATCH` で `public` と `private` を送れるボタンにする。他ユーザー確認は `switch-user` で header 値を `00000000-0000-4000-8000-000000000002` / `admin` と所有者の間で入れ替え、切替直後に現在の Namespace / Library slug で Library 詳細 GET を再実行して結果を `library-result` へ表示する。これにより private の 404 と public 化後の 200 を別ツールなしで確認できる。

`Makefile` の `run-api` は recipe が先に `cd backend` することを踏まえ、`FRONTEND_DIR="$(CURDIR)/frontend"` の絶対パスを子プロセスへ渡す。`.env.example` には空の `FRONTEND_DIR=` と、ローカル確認時はリポジトリ直下の `frontend` を使うコメントを足す。README には、この機能の手動確認が `docs/superpowers/plans/2026-11-08-library-sharing-mvp.md` の Task 7 Step 5 にあることだけを足す。

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/router ./internal/config ./internal/usecase ./internal/librarycombine -count=1`

Expected: PASS。

- [ ] **Step 5: Run the manual E2E checklist**

ローカル依存を起動する。

```sh
docker compose up -d
docker compose ps
make migrate-up
make seed
```

別ターミナルで API:

```sh
set -a
. ./.env
set +a
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export DEV_HEADER_AUTH=true
make run-api
```

別ターミナルで Worker:

```sh
set -a
. ./.env
set +a
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
cargo run --manifest-path judge/worker/Cargo.toml
```

Problem 1 の snapshot を README と同じく作る。

```sh
curl -sS -X PATCH http://localhost:8080/api/v1/problems/1/test-cases/1 \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 00000000-0000-4000-8000-000000000001' \
  -H 'X-User-Role: author' \
  -d '{"expected_output":"3\n"}'
```

ブラウザで `http://localhost:8080/` を開き、次を確認する。

1. dev auth で Namespace `seed-author` を作成できる。
2. private Library と、`def add(a, b): return a + b` を含む Version 1 を作成できる。
3. `switch-user` 後、その Library の GET が 404 になる。
4. 所有者に戻して visibility を public にすると、他ユーザーから Version 本文を読める。
5. 他ユーザーが problem `1` と adapter `a, b = map(int, input().split())` / `print(add(a, b))` で verify を開始できる。
6. ポーリングが `pending` または `running` から `ac` になり、画面に入力、期待出力、stdout、stderr が出ない。
7. 既存の `POST /api/v1/problems/1/submissions` が、library を使わない通常提出としても受け付けられる。
8. `judge/core` と `judge/worker` の git diff が空である。

Expected: 8 項目すべて成功。Worker 未起動で `ac` にならない場合は、Queue 設定を README の LocalStack 手順で確認し、判定基盤の不具合を library 実装で隠さない。

- [ ] **Step 6: Commit**

```bash
git add frontend/index.html frontend/app.js frontend/styles.css backend/internal/config/config.go backend/internal/router/router.go backend/internal/router/router_test.go backend/cmd/api/main.go Makefile .env.example README.md
git commit -m "feat: add same-origin manual library verification page"
```

## Spec Coverage

- ユーザー所有 Namespace: Task 3。
- Library と public/private の他ユーザー制御: Task 4。
- 不変 Version と hash: Task 2 と Task 5。
- 既存 Problem、adapter、Submission/Outbox/Judge: Task 1 と Task 6。
- 既存ジャッジ互換: Task 6 の Rust diff と cargo test。
- フロントからの手動確認: Task 7。
- Test Suite と group 所属を作らないこと: File Structure の変更禁止と、どの task にも対応するファイルが無いこと。

## Self-Review

- 仕様のエラー文言、hash、テンプレート、SQL 制約名は task から仕様の節を参照し、別の値を導入していない。
- Version 採番は SQL の `FOR UPDATE` に固定し、usecase の `count + 1` を許していない。
- verification は `SubmissionUsecase.Create` を迂回するが、payload 型は既存の `submissionOutboxPayload` を使う。
- 計画は実装本体を転記していない。結合テンプレートだけは、テストの期待値と仕様が決めるアルゴリズムとして Task 1 に置いた。
