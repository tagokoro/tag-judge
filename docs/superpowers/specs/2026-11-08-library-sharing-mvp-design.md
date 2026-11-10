# Library Sharing MVP 設計

- 日付: 2026-11-08
- 状態: **失効（2026-11-10）**。旧 adapter/Judge 検証方式の設計であり、実装の契約ではない。
- 実装計画: [docs/superpowers/plans/2026-11-08-library-sharing-mvp.md](../plans/2026-11-08-library-sharing-mvp.md)（同じく失効）
- 情報源: [docs/library-sharing-judge-requirements.md](../../library-sharing-judge-requirements.md)（同じく検証モデルは失効）
- 現行実装の基準: [README.md](../../../README.md) と main のコード。設計資料の初期構想より、動いている提出経路を優先する。

> ⚠️ **失効（2026-11-10）— この設計書は提出URL方式へ置き換えられました**
>
> 本設計書は adapter と Library Version のコードを結合し、既存 Submission / Outbox / Judge で検証する旧方式の契約です。**[提出URL設計](2026-11-10-verification-url-design.md) と [提出URL実装計画](../plans/2026-11-10-verification-url.md) が新しい唯一の契約**です。
>
> - `library_verifications` は `id, library_version_id, created_by, submission_url, created_at` だけを持ち、`submission_id` / `adapter_source` / `adapter_sha256` は削除。
> - URL は絶対 HTTP(S)（ホストあり・制御文字なし・2048バイト以内）。取得・解析・判定はしない。**Submission / Outbox / S3 / Judge は作成しない**。
> - リンクは自己宣言の「関連する提出」であり検証済みの証明ではない。verdict / score / status / case_results / adapter は存在しない。
> - 登録は認証済み利用者（public は誰でも、private は所有者のみ）。閲覧は現在の Library 公開範囲に追従する。
>
> よって本設計書の §1 目的、§2 合意済み範囲、§5 主要決定、§6.1 up SQL（旧 `library_verifications`）、§7 コード結合ルール、§8 既存 Judge への接続、§9 認可の verify 行、§10.4 Verification、§11.1 画面の adapter / polling 要素、§13 テスト、§14 リスク、§15 / §16 の検証・判定に関する**全記述と受け入れ条件は無効**です。特に §13 の「verification の outbox payload」「insert failure の rollback」、§16 の「outbox の JSON キー」「結合テンプレート / fixture」は現在の受け入れ条件ではありません。Namespace / Library / Library Version の不変性など、検証と無関係な部分は引き続き参考になります。

## 1. 目的

ユーザーが自分の Namespace に Python ライブラリを置き、公開または非公開を切り替え、不変の Library Version を作る。利用可能な既存 Problem を選び、その問題用 adapter と Library Version のコードを結合し、既存の Submission / Outbox / Judge で検証する。既存の問題提出と判定を壊さない。

手動確認は、Next.js App Router + TypeScript のフロントエンドから行う。Next.js は `localhost:3000` の独立プロセス、Go API は `localhost:8080` とし、ブラウザからは Next.js の same-origin `/api/:path*` を利用する。

## 2. 合意済み範囲

実装するものは次だけである。

- ユーザー所有の Namespace。グループ所有は作らない。
- Namespace 配下の Library。
- Library の `public` / `private` と、他ユーザーからのアクセス制御。
- 作成後に内容を変えられない Library Version。
- 既存 Problem を選び、Library Version のコードと問題別 adapter を既存 Submission / Outbox / Judge へ流して verify する。
- 既存ジャッジとの互換。Worker と judge-core は変更しない。
- バックエンド API と、フロントからの手動動作確認。

## 3. 対象外

次は実装しない。要件文書にあっても、このMVPでは追加しない。

- Test Suite、Test Suite Version、Problem Version、共通 Judge Run への置換。
- Library または Namespace の group 所属、グループ権限、`unlisted`。
- draft / publish / yank、フォーク、ライセンス選択、出典、参照ヘッダー。
- API Token、AI 操作、Change Request、監査ログ、検索、タグ、スター。
- 複数ファイル、依存関係解決、複数言語、Special Judge、インタラクティブ問題。
- 検証結果の公開バッジ、ライブラリ所有者による他人の adapter 閲覧。
- private / group Problem への新規提出経路。
- Submission 一覧から検証提出を隠すこと。
- Vite、Playwright、CORS、新しい認証方式。Next.js はこのMVPのフロント実装に必要な範囲で採用する。

## 4. 現行リポジトリの確認

- フロントエンドは存在しない。`package.json` も `frontend/` もない。README は「フロントエンドは main には含まれていません」と書いている。
- `AGENTS.md` は存在しない。
- API は Go、`chi`、PostgreSQL、sqlc。判定は Rust `judge-core` と `judge-worker`。
- 提出は `SubmissionUsecase.Create` がソースを storage へ置き、`CreateWithOutbox` が `submissions` と `submission_job_outbox` を同一トランザクションで保存する。Publisher が既存の SQS へ送る。
- DirectRunner も IsolateRunner も、渡されたソースを `main.py` 1 ファイルとして `python3` で実行する。追加モジュールの配置も `PYTHONPATH` もない。
- 提出できる Problem は `FindPublicByID` で見える public problem だけで、`current_test_cases_key` が空なら `409 PROBLEM_HAS_NO_TEST_CASES` になる。
- 認証は session cookie。ローカルでは `DEV_HEADER_AUTH=true` のとき `X-User-ID` と `X-User-Role` を使う。API に CORS はない。
- seed ユーザーは author `00000000-0000-4000-8000-000000000001` と admin `00000000-0000-4000-8000-000000000002`。Problem 1 は public の A+B だが、seed はテストケース snapshot を作らない。

## 5. 主要決定

1. 「公開/非公開」は Library の visibility であり、Version の draft/publish ではない。Version は作成時点で不変である。
2. ユーザーは Namespace を 1 つだけ持てる。slug は作成時にユーザーが指定し、その後変えない。
3. Library Version は Python ソース文字列 1 本である。複数ファイルは既存ジャッジが実行できないため入れない。
4. adapter は Version の一部ではない。verify のたびに問題別の文字列として渡し、検証レコードに保存する。
5. 結合は API 内の純関数で行い、結果の 1 本の Python を通常の提出ソースとして既存経路へ入れる。Worker、core、Outbox の JSON、Queue は変えない。
6. private Library は所有者以外には存在を見せない。admin も例外にしない。public の読み取りと verify は他ユーザーにも許す。書き込みは所有者だけである。
7. 検証結果と adapter は、その検証を開始したユーザー、または既存の提出詳細と同じく admin だけが読める。
8. 検証提出は通常の `submissions` 行である。`GET /api/v1/submissions` に含まれる。kind 列は足さない。
9. フロントは Next.js App Router + TypeScript とし、Go API とは独立プロセスで動かす。Next.js の rewrites により browser-facing origin の `/api/:path*` を server-side の `API_SERVER_ORIGIN`（開発時は `http://localhost:8080`）へ proxy する。

10. 認証は既存の Go API の HttpOnly、SameSite=Lax cookie を使う。OAuth callback も `/api/v1/auth/google/callback` として Next.js rewrite 経由で Go API に到達させ、cookie は browser-facing Next origin に設定される。

## 6. データモデル

migration は `backend/db/migrations/000009_create_library_sharing.up.sql` と対応する down とする。現行 main の `000007_create_admission_control` と `000008_create_code_run_job_outbox` を保持し、その次に追加する。origin/main の admission-control 変更を上書き・巻き戻ししない。

既存の `users`、`problems`、`submissions`、`submission_job_outbox`、`submission_results` は変更しない。

```text
users 1 ── 1 namespaces 1 ── * libraries 1 ── * library_versions 1 ── * library_verifications
users 1 ── * submissions 1 ── 0..1 library_verifications
problems 1 ── * submissions
```

グループテーブルへ外部キーを張らない。

### 6.1 up SQL

実装はこの SQL をそのまま使う。

```sql
CREATE TABLE namespaces (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users (id),
    slug text NOT NULL,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT namespaces_owner_user_unique UNIQUE (owner_user_id),
    CONSTRAINT namespaces_slug_unique UNIQUE (slug),
    CONSTRAINT namespaces_slug_check CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    CONSTRAINT namespaces_slug_length_check CHECK (char_length(slug) BETWEEN 2 AND 32),
    CONSTRAINT namespaces_display_name_check CHECK (char_length(btrim(display_name)) BETWEEN 1 AND 64)
);

CREATE TABLE libraries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace_id uuid NOT NULL REFERENCES namespaces (id),
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    visibility text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT libraries_namespace_slug_unique UNIQUE (namespace_id, slug),
    CONSTRAINT libraries_slug_check CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    CONSTRAINT libraries_slug_length_check CHECK (char_length(slug) BETWEEN 2 AND 48),
    CONSTRAINT libraries_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 80),
    CONSTRAINT libraries_description_length_check CHECK (char_length(description) <= 2000),
    CONSTRAINT libraries_visibility_check CHECK (visibility IN ('public', 'private'))
);

CREATE INDEX libraries_namespace_visibility_idx
    ON libraries (namespace_id, visibility, created_at DESC, slug ASC);
CREATE INDEX libraries_namespace_created_idx
    ON libraries (namespace_id, created_at DESC, slug ASC);

CREATE TABLE library_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id uuid NOT NULL REFERENCES libraries (id),
    version_number integer NOT NULL,
    source_code text NOT NULL,
    content_sha256 text NOT NULL,
    byte_size integer NOT NULL,
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT library_versions_library_number_unique UNIQUE (library_id, version_number),
    CONSTRAINT library_versions_number_check CHECK (version_number >= 1),
    CONSTRAINT library_versions_source_check CHECK (btrim(source_code) <> ''),
    CONSTRAINT library_versions_sha_check CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT library_versions_sha_matches_check CHECK (
        content_sha256 = encode(digest(convert_to(source_code, 'UTF8'), 'sha256'), 'hex')
    ),
    CONSTRAINT library_versions_byte_size_check CHECK (
        byte_size = octet_length(source_code) AND byte_size BETWEEN 1 AND 100000
    )
);

CREATE INDEX library_versions_created_by_idx ON library_versions (created_by);

CREATE FUNCTION reject_library_version_mutation() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'library_versions are immutable';
END;
$$;

CREATE TRIGGER library_versions_no_update_delete
    BEFORE UPDATE OR DELETE ON library_versions
    FOR EACH ROW
    EXECUTE FUNCTION reject_library_version_mutation();

CREATE TABLE library_verifications (
    id bigserial PRIMARY KEY,
    library_version_id uuid NOT NULL REFERENCES library_versions (id),
    submission_id bigint NOT NULL REFERENCES submissions (id),
    adapter_source text NOT NULL,
    adapter_sha256 text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT library_verifications_submission_unique UNIQUE (submission_id),
    CONSTRAINT library_verifications_adapter_check CHECK (btrim(adapter_source) <> ''),
    CONSTRAINT library_verifications_adapter_size_check CHECK (
        octet_length(adapter_source) BETWEEN 1 AND 100000
    ),
    CONSTRAINT library_verifications_adapter_sha_check CHECK (adapter_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT library_verifications_adapter_sha_matches_check CHECK (
        adapter_sha256 = encode(digest(convert_to(adapter_source, 'UTF8'), 'sha256'), 'hex')
    )
);

CREATE INDEX library_verifications_version_idx
    ON library_verifications (library_version_id);
CREATE INDEX library_verifications_created_idx
    ON library_verifications (created_at DESC, id DESC);
```

`problem_id` と検証開始者は `submission_id` で参照する既存 `submissions.problem_id` / `submissions.user_id` を正とし、`library_verifications` へ重複保存しない。これにより、検証レコードと実際に Queue へ流れた Submission の問題・所有者が食い違う余地を作らない。一覧と詳細は必ず `submissions` を join してこれらを返す。

`library_versions` に `updated_at` は置かない。更新 API も作らない。不変性は API の欠如と trigger の両方で守る。

### 6.2 down SQL

依存の逆順で消す。

```sql
DROP TABLE IF EXISTS library_verifications;
DROP TRIGGER IF EXISTS library_versions_no_update_delete ON library_versions;
DROP FUNCTION IF EXISTS reject_library_version_mutation();
DROP TABLE IF EXISTS library_versions;
DROP TABLE IF EXISTS libraries;
DROP TABLE IF EXISTS namespaces;
```

### 6.3 Version 番号

`version_number` はシステムが 1 から割り当てる。同一 library 内で一意である。利用者指定の semver は受け取らない。

採番は library 行の `SELECT ... FOR UPDATE` のあとに `MAX(version_number) + 1` を使う。sqlc クエリは次の形にする。

```sql
-- name: CreateLibraryVersion :one
WITH locked AS (
    SELECT id
    FROM libraries
    WHERE id = sqlc.arg(library_id)::uuid
    FOR UPDATE
)
INSERT INTO library_versions (
    library_id,
    version_number,
    source_code,
    content_sha256,
    byte_size,
    created_by
)
SELECT
    locked.id,
    COALESCE((
        SELECT MAX(version_number)
        FROM library_versions
        WHERE library_id = locked.id
    ), 0) + 1,
    sqlc.arg(source_code),
    sqlc.arg(content_sha256),
    sqlc.arg(byte_size),
    sqlc.arg(created_by)::uuid
FROM locked
RETURNING
    id::text AS id,
    library_id::text AS library_id,
    version_number,
    source_code,
    content_sha256,
    byte_size,
    created_by::text AS created_by,
    created_at;
```

`content_sha256` は保存する `source_code` の UTF-8 バイト列の SHA-256 を小文字 hex にした値である。前後空白は、空判定以外では削らない。

## 7. コード結合ルール

結合は `librarycombine.Combine(librarySource, adapterSource string) (string, error)` が行う。Python は実行しない。

### 7.1 検証

library と adapter の両方に同じバイト制約を適用する。

- `strings.TrimSpace` が空なら拒否する。保存文字列自体は trim しない。
- バイト列に `NUL` (`\x00`) を含めない。
- UTF-8 バイト長は 1 以上 100000 以下。JSON デコード済みの Go string を `len([]byte(source))` で測る。
- 言語は Python だけである。フィールドは受け取らない。

拒否時の sentinel は `ErrEmptySource`、`ErrNULSource`、`ErrSourceTooLarge` である。sentinel 自体は library / adapter のどちらで失敗したかを表さないため、Version 作成 usecase は保存前に library を、Verification 作成 usecase は結合前に adapter をそれぞれ `ValidateSource` し、library 側を `INVALID_SOURCE_CODE`、adapter 側を `INVALID_ADAPTER_SOURCE` に変換する。その後 `Combine` でも両方を防御的に再検証する。メッセージは次で固定する。

- `source_code must be 1-100000 bytes, non-blank, and must not contain NUL`
- `adapter_source must be 1-100000 bytes, non-blank, and must not contain NUL`

### 7.2 呼び出し契約

- adapter は library を `import` しない。library のトップレベルで定義した関数とクラスは、adapter のグローバルから直接呼ぶ。
- library は stdin を読まず、解答を stdout に書かない。トップレベルで `input()` を呼ぶと、その分だけ adapter より先に stdin を消費する。これは単一プロセスの逐次 `exec` であり、修正対象にしない。
- library 内の `if __name__ == "__main__"` は実行しない。library の `__name__` は `"library"` にする。adapter の `__name__` は `"__main__"` にする。
- adapter が stdin を読み、stdout に既存ジャッジが比較する出力を書く。比較規則は現行の `trim_end` のままである。
- 名前が衝突したら adapter 側の代入が library の同名を隠す。呼び出し側の責任とする。

### 7.3 生成ソース

`encoding/base64` の `StdEncoding` で改行なしに符号化する。base64 文字に引用符はない。生成結果は次のテンプレートとバイト単位で一致させる。`{library_b64}` と `{adapter_b64}` だけを置き換える。

```python
import base64
_library = base64.b64decode('{library_b64}').decode("utf-8")
_adapter = base64.b64decode('{adapter_b64}').decode("utf-8")
_namespace = {"__name__": "library"}
exec(compile(_library, "<library>", "exec"), _namespace)
_namespace["__name__"] = "__main__"
exec(compile(_adapter, "<adapter>", "exec"), _namespace)
```

最後に改行を 1 つ付ける。

固定 fixture:

- library: `def add(a, b):\n    return a + b\n\nif __name__ == "__main__":\n    print("library-main")\n`
- SHA-256: `b24916acf9a8670a4e088af2bbc2591d02eca1f33ddad1aec0ed3a41d57568af`
- バイト長: 86
- adapter: `a, b = map(int, input().split())\nprint(add(a, b))\n`
- SHA-256: `b8d685a728f43ee0c53f5f299234c42d5ebbe0f423bb5b18d16a7ecaf2c03f21`
- バイト長: 50

この結合結果を `python3` に stdin `1 2\n` で渡すと、stdout は `3\n`、stderr は空、終了コードは 0 である。`library-main` は出力しない。

## 8. 既存 Judge への接続

verify は新しい Queue、新しい Worker メッセージ、新しい実行モードを作らない。

1. 呼び出し元が library を読めること、Version がその library のものであること、Problem が現行の public 提出条件を満たすことを確認する。
2. `Combine` の結果を、現行と同じ storage へ置く。キー形式は現行提出と同じ `submissions/{userID}/problems/{problemID}/{32 hex}.py` とする。
3. 1 つの DB トランザクションで Library 行を `FOR SHARE` で再取得し、`owner_user_id = actor.UserID` または `visibility = 'public'`、かつ Version がその Library に所属することを再確認する。その後、通常の `submissions` 行、現行と同じ `submission_job_outbox.payload`、`library_verifications` 行を挿入する。再確認に失敗した場合は全 DB 行を rollback し、private / 不存在と同じ `404 LIBRARY_NOT_FOUND` を返す。事前に保存済みの storage object は現行提出と同じ孤立 object として許容する。
4. 既存の `SubmissionOutboxPublisher` が payload を SQS へ送る。Worker は通常の problem execution として `main.py` を判定する。
5. クライアントは検証詳細、または既存の `GET /api/v1/submissions/{submission_id}` をポーリングする。ケース結果の秘匿は現行詳細 API と同じである。

payload は `postgres.submissionOutboxPayload` をそのまま `json.Marshal` する。新しい struct を作らない。フィールドは次だけである。

```json
{
  "submissionId": "15",
  "language": "python",
  "sourceCodeKey": "submissions/00000000-0000-4000-8000-000000000001/problems/1/0123456789abcdef0123456789abcdef.py",
  "testCasesKey": "problems/1/testcases/abc.json",
  "timeLimitMs": 2000
}
```

`submissionId` は挿入後の 10 進 ID 文字列である。`language` は `python`、`status` は `pending`、`score` は 0 である。`timeLimitMs` は Problem の `time_limit_ms`、`testCasesKey` は提出時点の `current_test_cases_key` である。

storage 保存が成功して DB が失敗した場合、現行提出と同じく孤立オブジェクトを残してエラーを返す。削除ジョブは足さない。POST の再試行は別の Submission と別の verification を作る。冪等キーは足さない。

library を後から private にしても、すでに作られた提出と判定は取り消さない。ソースは提出時のオブジェクトに固定されている。新規 verify とソースの再取得だけを拒否する。

## 9. 認可

未認証は actor なしである。所有者は Namespace の `owner_user_id` と actor の user ID が一致するユーザーである。admin ロールは private Library を開ける特別権限ではない。admin が検証詳細を読めるのは、現行の提出詳細が admin に開いているのと同じ理由だけである。

| 操作 | 未認証 | 他ユーザー | 所有者 | admin（非所有者） |
| --- | --- | --- | --- | --- |
| Namespace 作成、自分の Namespace 取得 | 401 | 自分の分だけ | 可 | 自分の分だけ |
| Namespace の公開プロフィール取得 | 可 | 可 | 可 | 可 |
| Library 作成、更新、Version 作成 | 401 | public なら 403、private または不存在なら 404 | 可 | public なら 403、private なら 404 |
| public Library と Version の読み取り | 可 | 可 | 可 | 可 |
| private Library と Version の読み取り | 404 | 404 | 可 | 404 |
| public Library の verify | 401 | 可 | 可 | 可 |
| private Library の verify | 401 | 404 | 可 | 404 |
| 自分の verification 取得 | 401 | 403 | 可 | 可 |
| 他人の verification 一覧 | 401 | 自分の一覧だけ | 自分の一覧だけ | 自分の一覧だけ |

private を 403 にしない。存在を漏らさないため、不存在と同じ `404 LIBRARY_NOT_FOUND` / `library not found` を返す。public を読める非所有者が更新または Version 作成をした場合だけ `403 LIBRARY_FORBIDDEN` / `library owner required` を返す。

verify の読み取り認可だけを usecase の事前確認に頼らない。所有者が同時に public から private へ変更した場合でも、節 8 の DB トランザクション内再確認後に新しい Submission / Outbox / verification を commit してはならない。

検証に使える Problem は、現行 `FindPublicByID` が返し、`current_test_cases_key` が空白でないものだけである。private Problem は `404`、テストケース未生成は `409 PROBLEM_HAS_NO_TEST_CASES` / `problem has no test cases` とする。

## 10. API 契約

ベースは `/api/v1`。成功は現行どおり `{"data": ...}`、失敗は `{"error":{"code":"...","message":"..."}}` である。JSON は snake_case、未知フィールドは拒否、body は 1 個だけである。時刻は Go の `time.Time` JSON エンコードに合わせる。認証が要るルートは現行の `RequireAuth` を付ける。

一覧の `limit` は省略時 20、1 から 100 以外は `400 INVALID_LIMIT` / `limit must be between 1 and 100` である。`offset` が負なら `400 INVALID_OFFSET` / `offset must be greater than or equal to 0` である。

### 10.1 Namespace

`POST /api/v1/namespaces`

```json
{ "slug": "seed-author", "display_name": "Seed Author" }
```

`201`:

```json
{
  "data": {
    "id": "uuid",
    "slug": "seed-author",
    "display_name": "Seed Author",
    "owner_user_id": "00000000-0000-4000-8000-000000000001",
    "created_at": "2026-11-08T00:00:00Z",
    "updated_at": "2026-11-08T00:00:00Z"
  }
}
```

- slug は `^[a-z0-9]+(?:-[a-z0-9]+)*$` かつ 2 から 32 文字。違反は `400 INVALID_SLUG` / `slug must be 2-32 lowercase letters, digits, or single hyphens`。
- display_name は trim 後 1 から 64 文字。保存値は trim 後。違反は `400 INVALID_DISPLAY_NAME` / `display_name must be 1-64 characters`。
- 同一ユーザーの 2 件目は `409 NAMESPACE_ALREADY_EXISTS` / `user already has a namespace`。制約名 `namespaces_owner_user_unique` で判定する。
- slug 衝突は `409 NAMESPACE_SLUG_TAKEN` / `namespace slug is already taken`。制約名 `namespaces_slug_unique` で判定する。

`GET /api/v1/me/namespace` は認証必須。無ければ `404 NAMESPACE_NOT_FOUND` / `namespace not found`。

`GET /api/v1/namespaces/{namespace_slug}` は認証不要。同じ 404 を使う。更新と削除のルートは作らない。

### 10.2 Library

`POST /api/v1/namespaces/{namespace_slug}/libraries`

```json
{
  "slug": "union-find",
  "name": "UnionFind",
  "description": "",
  "visibility": "private"
}
```

`201` の data:

```json
{
  "id": "uuid",
  "namespace_slug": "seed-author",
  "slug": "union-find",
  "name": "UnionFind",
  "description": "",
  "visibility": "private",
  "created_at": "2026-11-08T00:00:00Z",
  "updated_at": "2026-11-08T00:00:00Z"
}
```

- slug は同じ正規表現で 2 から 48 文字。`400 INVALID_SLUG` / `slug must be 2-48 lowercase letters, digits, or single hyphens`。
- name は trim 後 1 から 80 文字。保存値は trim 後。`400 INVALID_NAME` / `name must be 1-80 characters`。
- description は空を許し、2000 文字以下。trim しない。`400 INVALID_DESCRIPTION` / `description must be at most 2000 characters`。
- visibility は `public` か `private` のみ。省略とそれ以外は `400 INVALID_VISIBILITY` / `visibility must be public or private`。
- slug 衝突は `409 LIBRARY_SLUG_TAKEN` / `library slug is already taken`。制約名 `libraries_namespace_slug_unique`。
- Namespace が無い、または private 相当の隠れた Library への操作は `404`。ここでの Namespace 不存在は `NAMESPACE_NOT_FOUND`。

`GET /api/v1/namespaces/{namespace_slug}/libraries` はページングする。所有者は public と private の両方、それ以外は public だけを `created_at DESC, slug ASC` で返す。data は配列である。

`GET /api/v1/namespaces/{namespace_slug}/libraries/{library_slug}` は 1 件を返す。

`PATCH` は同じパスで、`name`、`description`、`visibility` の任意の部分集合を受ける。1 つも無ければ `400 EMPTY_UPDATE` / `at least one field is required`。slug は変えない。未知フィールドは拒否されるので slug を送ると `400 INVALID_REQUEST_BODY` になる。所有者以外の成否は認可表に従う。成功は `200`。

Library の削除ルートは作らない。隠す操作は visibility を `private` にすることである。

### 10.3 Library Version

`POST /api/v1/namespaces/{namespace_slug}/libraries/{library_slug}/versions`

```json
{ "source_code": "def add(a, b):\n    return a + b\n" }
```

所有者だけが `201` を受け取る。data は `id`、`version_number`、`source_code`、`content_sha256`、`byte_size`、`created_at` を含む。`version_number` は 1 始まりである。

`GET .../versions` は読める人に要約配列を返す。各要素は `id`、`version_number`、`content_sha256`、`byte_size`、`created_at` だけで、`source_code` を含めない。`version_number DESC` で、節 10 共通の `limit` / `offset` によりページングする。

`GET .../versions/{version_number}` は読める人に `source_code` を含めて返す。数値でない version は `400 INVALID_VERSION_NUMBER` / `version_number must be a positive integer`。無い Version は `404 LIBRARY_VERSION_NOT_FOUND` / `library version not found`。

`PATCH` と `DELETE` は登録しない。chi が `405` を返す。

### 10.4 Verification

`POST .../versions/{version_number}/verifications`

```json
{
  "problem_id": 1,
  "adapter_source": "a, b = map(int, input().split())\nprint(add(a, b))\n"
}
```

読める library に対して認証済みユーザーが開始できる。`201`:

```json
{
  "data": {
    "id": 1,
    "library_version_id": "uuid",
    "namespace_slug": "seed-author",
    "library_slug": "union-find",
    "version_number": 1,
    "problem_id": 1,
    "submission_id": 15,
    "status": "pending",
    "score": 0,
    "adapter_sha256": "b8d685a728f43ee0c53f5f299234c42d5ebbe0f423bb5b18d16a7ecaf2c03f21",
    "created_at": "2026-11-08T00:00:00Z",
    "judged_at": null
  }
}
```

作成レスポンスに `adapter_source` と `case_results` は含めない。

`problem_id` が 0 以下なら `400 INVALID_PROBLEM_ID` / `problem_id must be a positive integer`。`version_number` が数値でない、0 以下、または Go の `int` に収まらない場合は節 10.3 と同じ `400 INVALID_VERSION_NUMBER` とする。

`GET /api/v1/library-verifications/{verification_id}` は開始者と admin が読める。data は作成レスポンスに `adapter_source` と `case_results` を加えたものである。`case_results` の要素は現行提出詳細と同じ `test_case_id`、`status`、`execution_time_ms`、`memory_kb` だけである。`pending` と `running` は空配列である。stdout、stderr、入力、期待出力は返さない。

不存在は `404 LIBRARY_VERIFICATION_NOT_FOUND` / `library verification not found`。存在して権限が無い場合は `403 LIBRARY_VERIFICATION_FORBIDDEN` / `library verification is not accessible`。

`verification_id` が数値でなければ `400 INVALID_LIBRARY_VERIFICATION_ID` / `verification_id must be an integer`、0 以下なら同じ code で `verification_id must be a positive integer` とする。

`GET /api/v1/library-verifications` は認証ユーザー自身の検証を `created_at DESC, id DESC` でページングする。一覧要素に `adapter_source` と `case_results` は含めない。

## 11. Next.js フロント構成

フロントは `frontend/` の独立した Next.js App Router + TypeScript プロジェクトとする。PR3 で `create-next-app@latest` の現行安定版を使って scaffold し、生成時点の package lock をコミットする。Next.js の公式ドキュメントが要求する最小 Node.js バージョンを採用し、PR3 の `package.json` と README に明記する（npm package の具体的なバージョンは PR3 で解決・固定する）。

Go API は `localhost:8080`、Next.js は `localhost:3000` の独立プロセスである。`frontend/next.config.*` の公式 rewrites パターンで、browser の `/api/:path*` を server-side `process.env.API_SERVER_ORIGIN`（開発既定 `http://localhost:8080`）へ proxy する。`/api/v1/auth/google/callback` も同じ rewrite に含めるため、Google の redirect URI は `http://localhost:3000/api/v1/auth/google/callback` とする。Go の既存 `GoogleOAuthRedirectURL` はこの URI、`AuthSuccessRedirectURL` は `http://localhost:3000/` として設定する。

既存の Go API 認証と OAuth 実装は変更しない。Go は callback の成功時に HttpOnly、SameSite=Lax cookie を browser-facing Next origin へ設定し、ブラウザの client component は `credentials: 'same-origin'` の fetch で cookie を利用する。API_SERVER_ORIGIN への server-side proxy は cookie forwarding を維持する。CORS は追加しない。

初回の scaffold では root layout と placeholder page、scripts、proxy、env/local Make target のみを置く。Go 側の `FRONTEND_DIR` と静的ファイル serving は削除する。PR4 で実際の namespace/library/version/verify UI を実装する。

### 11.1 画面

ページは日本語のラベルを使う。必須 id は次である。

- `dev-user-id`、`dev-user-role`、`use-dev-auth`
- `namespace-slug`、`namespace-display-name`、`create-namespace`、`namespace-result`
- `library-slug`、`library-name`、`library-description`、`library-visibility`、`create-library`、`library-result`
- `version-source`、`create-version`、`version-result`
- `problem-id`、`adapter-source`、`start-verification`、`verification-result`
- `switch-user`。他ユーザー確認用に、dev header の値を入れ替えて同じ操作を再実行できること

初期値は seed の手動確認に合わせる。user id は `00000000-0000-4000-8000-000000000001`、role は `author`、problem id は `1`、visibility の初期選択は `private` である。

動作:

1. `credentials: 'same-origin'` で `GET /api/v1/me` する。成功したら cookie セッションを使い、dev header を送らない。
2. 失敗し、利用者が dev auth を選んだ場合だけ `X-User-ID` と `X-User-Role` を送る。画面に「`DEV_HEADER_AUTH=true` のローカル専用」と表示する。
3. Google ログインは既存の `GET /api/v1/auth/google/login` へのリンクだけにする。新しい OAuth 実装はしない。
4. Namespace 作成、Library 作成、visibility の PATCH、Version 作成、verification 開始、検証詳細のポーリングを節 10 のパスで行う。
5. ポーリングは 1 秒間隔で最大 30 回、`GET /api/v1/library-verifications/{id}` を呼ぶ。終端 status は `ac`、`wa`、`tle`、`mle`、`re`、`ce`、`ie` である。
6. Library 作成後は GET 詳細を再取得する操作を用意する。`switch-user` は dev header の値を切り替えた直後に現在の Library 詳細を再取得し、private なら 404、所有者が public に変えたあとは同じ操作で 200 になることを同じページで確認できる。

ブラウザ自動テストは MVP では足さない。Next.js の型・lint/build と API の Go テストで契約を検証し、操作確認は節 13 の手動手順で行う。

## 12. エラーと互換

新しいエラー種別は `apperror` に足さない。既存の InvalidInput、Unauthorized、Forbidden、NotFound、Conflict を使う。

変えないもの:

- `judge/core/**`、`judge/worker/**`
- `submissions` と `submission_job_outbox` のスキーマ
- `submissionOutboxPayload` のフィールド
- 既存提出 API のリクエストとレスポンス
- Problem、Test Case、Group、CodeRun、認証の挙動

`docs/library-sharing-judge-requirements.md` は変更しない。

## 13. テストと手動確認 <!-- 失効: 検証・判定の受け入れ条件は 2026-11-10 提出URL設計で無効 -->

> **失効:** この節の自動テスト・手動 E2E のうち verification / outbox / adapter / polling / `ac` 判定に関する受け入れ条件は無効です。現行の受け入れ条件は [提出URL設計](2026-11-10-verification-url-design.md) と [提出URL実装計画](../plans/2026-11-10-verification-url.md) を参照してください。

実装は RED-GREEN-REFACTOR で進める。失敗するテストを先に書き、期待した理由で失敗することを確認してから、そのテストを通す最小実装を書く。

自動テスト:

- 結合 fixture のバイト一致と、`python3` による stdout `3\n`。
- 認可表の usecase テスト。private は admin にも 404。
- Version 更新メソッドが無く、HTTP の PATCH/DELETE が 405 であること。
- PostgreSQL 統合テストが trigger の `library_versions are immutable` を確認すること。
- PostgreSQL 統合テストが Version の同時作成を行い、重複せず連番が割り当てられること。
- verification の outbox payload が節 8 のキーだけを持つこと。
- verification insert が失敗したとき、同じトランザクションの Submission と Outbox も残らないこと。
- public から private への同時変更を DB 内で再確認し、非所有者の新規 verification を commit しないこと。
- public Library を読める他ユーザーが、他人の `adapter_source` を受け取れないこと。
- Next.js rewrite 経由でも `GET /api/v1/problems` が Go API に到達すること、OAuth callback rewrite が同じ origin で到達すること。

統合テストは既存の `SUBMISSION_DETAILS_TEST_DATABASE_URL` を使う。CI はこの変数を設定済みなので、新しい統合テストも同じ条件で実行される。未設定のローカル `make test` では skip する。既存の提出詳細テストは `*.up.sql` を字句順に適用するため、`000009` は `000008` の後に適用される。

手動 E2E は、LocalStack、Go API、Next.js、Worker を起動した状態で行う。seed の Problem 1 は、README と同じく先にテストケースを更新して `current_test_cases_key` を作る。その後 Next.js origin から private 作成、他ユーザー 404、public 化、Version 作成、A+B adapter の verify、`ac` までのポーリングを確認する。DB/Worker/LocalStack がない環境では E2E は実行できず、API/usecase のテストと Next build を代替の受入れ条件とする。

## 14. リスク

- 単一ファイル結合のため、`import library` と複数ファイルは動かない。これは互換を守るための意図した制限である。
- library のトップレベル `input()` は adapter より先に stdin を消費する。呼び出し契約で回避し、実行基盤では分離しない。
- 検証提出が通常の提出一覧に出る。隠す変更は既存一覧の意味を変えるのでしない。
- storage 成功後の DB 失敗で孤立オブジェクトが残る。現行提出と同じ受容済みの隙間である。
- POST 再試行は二重判定になる。現行提出と同じである。
- private 化の後も実行中ジョブは判定を完了する。提出時 snapshot を守るためである。
- デフォルトの `make test` は DB なしでは trigger を実行しない。CI の統合テストと手動 migration がそれを補う。
- Next.js UI は XSS を考慮し、API のソースを unsafe HTML として描画しない。
- dev header を本番で有効にしない。フロントはそれをローカル専用と表示する。

## 15. PR 分割と未解決事項

依存順に、次の 4 つの独立してレビュー可能な stacked PR とする。各 PR は前の PR を base にする。

1. **PR1 — backend DB/schema/persistence**: migration、sqlc query/generated、domain/repository、PostgreSQL persistence と integration tests。API route、handler、usecase、OpenAPI、既存 judge integration は含めない。
2. **PR2 — API/usecase/handler/OpenAPI/existing judge integration**: librarycombine、usecase、handler、chi router、OpenAPI、main wiring、API/usecase tests と existing Submission/Outbox/Judge integration。Worker/core は変更しない。
3. **PR3 — frontend environment setup**: Next.js scaffold（App Router + TypeScript）、package lock、tsconfig/eslint/next config、root layout/placeholder、scripts、rewrites、env docs/local Make targets。Go 側の `FRONTEND_DIR` と静的 serving は除去する。UI は placeholder のままにする。
4. **PR4 — Next.js UI implementation**: namespace/library/version/verify の画面、cookie session fetch、明示選択時だけの dev header auth、OAuth link、polling、UI tests/typecheck/build。PR3 の proxy を利用する。

Google OAuth は既存 config の `GoogleOAuthRedirectURL` / `AuthSuccessRedirectURL` を PR3 の local env docs で Next origin に設定する。production の hostname、secret、cookie domain の具体値は deployment 側で決める。

実装を止める未決事項はない。次は合意範囲から意図的に外したので、実装中に広げない。

- 要件文書にある複数ファイル、Test Suite、グループ Namespace、ライセンス、フォーク、API Token を、このMVPへ戻すかどうか。戻す場合は別設計にする。
- public Library のページに他人の検証結果を出すかどうか。現契約では出さない。
- 著者が自分の private Problem で自分の Library を検証できるようにするかどうか。現契約では現行提出条件を広げない。
- ユーザー指定の version 文字列を将来足すかどうか。現契約はシステム採番だけである。

## 16. グローバル制約 <!-- 失効: adapter/結合/outbox/判定の制約は 2026-11-10 提出URL設計で無効 -->

> **失効:** この節の「Library と adapter の上限」「結合テンプレート / fixture / outbox の JSON キー」「既存提出スキーマは変更しない」など検証方式に依存する制約は無効です。現行の制約は [提出URL設計](2026-11-10-verification-url-design.md) を参照してください。

- Go module は `github.com/tagokoro/tag-judge/backend`、Go 1.26、PostgreSQL 16、Python 3 のみ。
- Library と adapter の上限はそれぞれ 100000 バイト。Version の DB check も 100000 である。
- slug 正規表現は `^[a-z0-9]+(?:-[a-z0-9]+)*$`。Namespace は 2 から 32、Library は 2 から 48。
- visibility は `public` と `private` だけ。
- 結合テンプレート、fixture の hash、outbox の JSON キーは節 7 と節 8 の値から変えない。
- エラーコードとメッセージは節 9 と節 10 の文言から変えない。
- `judge/core`、`judge/worker`、要件文書、既存提出スキーマは変更しない。
