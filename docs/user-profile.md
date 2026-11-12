# ユーザープロフィール 仕様

ユーザーごとのハンドル・表示名・外部リンクと、プロフィールページ（Overview / Libraries / Problems）の仕様です。
Namespace と Library の仕様は [ライブラリ共有 仕様](library-sharing.md)、HTTP API の詳細は [OpenAPI](../backend/docs/openapi.yaml) を正とします。

## 目的

- ユーザーを UUID ではなくハンドルで識別し、`/users/{handle}` でそのユーザーの Library と Problem をまとめて見られるようにする。
- 1 ユーザーが分野ごとに複数の Namespace（例：`graph`、`math`）を持てるようにする。
- ヘッダーのユーザーメニューに、プロフィールと設定への入口を置く。

## 用語

| 用語 | 説明 |
| --- | --- |
| ハンドル | ユーザーの公開 ID。2〜32 文字の英小文字・数字・単一ハイフン（`^[a-z0-9]+(?:-[a-z0-9]+)*$`）。全体で一意。一度決めたら変更できない |
| 表示名 | 画面に出す名前。前後の空白を除いて 1〜64 文字。いつでも変更できる。Namespace の名前とは別 |
| プロフィール | ハンドル・表示名・外部リンク。だれでも閲覧できる |

- Google アカウントの名前（`users.name`）とメールアドレスは、どの API・画面にも出さない。
- Namespace・Library・Problem・最近の提出・プロフィールの API は、ユーザーを UUID ではなくハンドルで表す。`GET /me` の `user_id`（自分自身の ID）はそのまま残す。Group の API（`created_by` などが UUID）は今回は変えない。

## データモデル

```sql
CREATE TABLE user_profiles (
    user_id uuid PRIMARY KEY REFERENCES users (id),
    handle text NOT NULL,            -- UNIQUE、ハンドルの規則を CHECK
    display_name text NOT NULL,      -- btrim して 1〜64 文字を CHECK
    atcoder_username text,           -- 外部リンク（第 2 段階）。NULL は未設定
    topcoder_username text,
    x_username text,
    github_username text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

- 公開 API の SQL は `user_profiles` を読み、`users` は読まない。メールアドレスが漏れる経路を作らないためである。
- `namespaces.owner_user_id` は `user_profiles (user_id)` を参照する。ハンドルのないユーザーは Namespace を持てない。
- マイグレーション `000010` で Namespace 関連の既存データを消してから、制約を作り直す。未リリースなので、既存データとの互換性や旧 URL（`/n/...`）のリダイレクトは用意しない。詳しくは [ライブラリ共有 仕様](library-sharing.md#マイグレーション) を参照。
- 開発用の seed（`backend/db/seeds/development.sql`）では、seed ユーザーにハンドル `seed-author` と `seed-admin` を付ける。

## 初回設定（ハンドルの作成）

1. Google ログインの callback で、ユーザーにハンドルがなければ `AUTH_ONBOARDING_REDIRECT_URL`（既定 `/settings/profile`）に転送する。ハンドルがあれば、これまでどおり `AUTH_SUCCESS_REDIRECT_URL` に転送する。
2. `/settings/profile` では、ハンドルがなければハンドルと表示名の入力欄を出す。表示名の欄は空のままにする（Google の名前を最初から入れておかない）。
3. 「保存」で `POST /me/profile` を呼び、成功したら `/users/{handle}` に移る。
4. ハンドルを作るまでの間も、閲覧と提出はできる。Namespace と Problem の作成だけは、API が 409 `HANDLE_REQUIRED` を返す。
5. ハンドルがない間、ユーザーメニューの最初の項目は「ハンドルを設定」（→ `/settings/profile`）にする。

## 画面と URL

| URL | 内容 | 描画 |
| --- | --- | --- |
| `/users/{handle}` | プロフィール + Overview タブ | Server Component |
| `/users/{handle}?tab=libraries` | Libraries タブ。`&namespace={slug}` で絞り込み、`&page={n}` でページ送り | Server Component |
| `/users/{handle}?tab=problems` | Problems タブ。`&page={n}` でページ送り | Server Component |
| `/users/{handle}/{namespace}` | Namespace のページ（[ライブラリ共有 仕様](library-sharing.md#画面と-url)） | Server Component |
| `/settings/profile` | ハンドルの作成（初回）、表示名と外部リンクの編集 | クライアント（ログイン必須） |
| `/settings/namespaces` | 自分の Namespace の一覧と作成 | クライアント（ログイン必須） |
| `/settings/namespaces/{namespace}` | Namespace の名前・説明の編集と削除 | クライアント（ログイン必須） |
| `/me` | ハンドルがあれば `/users/{handle}` へ、なければ `/settings/profile` へ転送 | クライアント |

- `/me/problems` と `/me/submissions` は今のまま残す。
- 閲覧ページと編集ページは分ける。プロフィールページには、本人にだけ濃い青の「編集」（→ `/settings/profile`）を出す。
- 存在しないハンドルは 404 にする。

### プロフィールページ `/users/{handle}`

```
┌───────────────────────────────────────────────┐
│ (頭文字)  表示名                        [編集] │  ← 編集は本人だけ
│           @handle   AtCoder ↗ X ↗ GitHub ↗     │  ← 設定したリンクだけ（第 2 段階）
└───────────────────────────────────────────────┘
  Overview   Libraries 12   Problems 3            ← 選択中は濃い青の下線
 ─────────────────────────────────────────────────
  （選択中のタブの内容）
```

- プロフィール部分はページの上部に置く。幅は `.page`（最大 880px）に合わせる。スマホ幅では縦に並べ、タブは横スクロールせずに収める。
- **タブ**は `?tab=` を変えるリンク（`<Link>`）で作り、選択中のタブに `aria-current="page"` を付ける。`role="tablist"` は使わない（矢印キーでの操作を実装しないため。#55 と同じ方針）。
- `tab` の値が想定外のときは Overview を出す。`page` が 1 以上の整数でないときは 1 ページ目として扱い、範囲外なら空の一覧を出す。どちらも 404 にはしない。
- **タブの件数**は `GET /users/{handle}` の `library_count` と `problem_count` を使う。数えるのは閲覧者が見られるものだけ（本人はすべて、他人は public のみ）。

#### Overview

| カード | 内容 |
| --- | --- |
| Libraries | 最後に更新された Library を 6 件。各行に Namespace の slug を付ける。右上に「すべて見る →」（`?tab=libraries`） |
| Problems | 最後に更新された Problem を 5 件。右上に「すべて見る →」（`?tab=problems`） |

- 中身が 0 件のカードは、空であることを短く書いて表示する（例：「public な Library はまだありません。」）。本人には作成への案内を出す。

#### Libraries タブ

- 上部に Namespace の絞り込みを横並びのボタンで出す（`すべて` + 閲覧者が見られる Namespace）。選択中のボタンに `aria-current="page"` を付ける。
- 一覧は `GET /users/{handle}/libraries?namespace=&limit=20&offset=` を使う。並びは最後に更新された順。
- 閲覧者が見られない Namespace（存在しない、または他人から見て public な Library がない）を `namespace=` に指定したときは、空の一覧を出す。
- 本人には右上に「Library を作成」（`/libraries/new`、絞り込み中なら `?namespace=` を付ける）と、各行に `public` / `private` のバッジを出す。

#### Problems タブ

- `GET /users/{handle}/problems?limit=20&offset=` を使う。並びは最後に更新された順。
- 本人には `private` / `group` を含めて出し、公開範囲のバッジを付ける。他人には `public` だけを出す。

### 設定ページ `/settings/profile`

- ハンドルがある場合、ハンドルは変更できない値として表示だけし、表示名を編集できるようにする。
- 外部リンク（第 2 段階）は、サービスごとに URL の前半（例：`https://atcoder.jp/users/`）を入力欄の前に出し、完成する URL が分かるようにする。
- フォームは `<form>` で作り、Enter キーでも送信できるようにする（#53）。「保存」で `PATCH /me/profile` を呼び、成功したら `/users/{handle}` に移る。

## 外部リンク（第 2 段階）

URL ではなく各サービスのユーザー名を保存し、URL はサーバーで組み立てて返す。任意の URL を受け付けないので、`javascript:` などのスキームによる悪用は起きない。

| サービス | 受け付ける形式 | 組み立てる URL |
| --- | --- | --- |
| AtCoder | `^[A-Za-z0-9_]{3,16}$` | `https://atcoder.jp/users/{username}` |
| TopCoder | 実装前に要確認（下記） | `https://profiles.topcoder.com/{username}`（パスとして encode する） |
| X | `^[A-Za-z0-9_]{1,15}$` | `https://x.com/{username}` |
| GitHub | `^[A-Za-z0-9](?:-?[A-Za-z0-9]){0,38}$` | `https://github.com/{username}` |

- 入力の前後の空白と、先頭の `@` を取り除いてから検証する。空文字か `null` は削除として扱う。
- 正となる検証は `domain` で行い、DB の CHECK は最後の守りとして置く（提出 URL と同じ方針）。違反は 400 `INVALID_ATCODER_USERNAME` / `INVALID_TOPCODER_USERNAME` / `INVALID_X_USERNAME` / `INVALID_GITHUB_USERNAME`。
- リンクは新しいタブで開き、`rel="noopener noreferrer nofollow"` を付ける。

## アバター

- 第 1 段階：表示名の先頭 1 文字を、水色の丸の中に出す（ヘッダーの今の `.user-avatar` と同じ部品を使う）。英字は大文字にする。読み上げでは表示名と重複するので `aria-hidden` にする。
- 第 2 段階（別の PR）：TagJudge に画像をアップロードできるようにする。
  - 受け付ける画像は PNG / JPEG / WebP で、1 MB まで。
  - サーバーで画像として読み込み直し、256×256 の PNG に変換して保存する。EXIF などのメタデータを消し、画像に見せかけた別のファイルも防ぐ。
  - 保存先は今ある S3 / LocalStack の `avatars/` 以下。Go の API から `Content-Type: image/png` と `X-Content-Type-Options: nosniff` を付けて配信する。
  - アップロードしていないユーザーには、第 1 段階と同じく頭文字を出す。

## ユーザーメニュー

ヘッダー右のボタンには、頭文字のアバター・表示名（ハンドルがなければ「ハンドル未設定」）・ロールを出す。PC のメニューとスマホの `PhoneSheet` の項目は同じにする。

```
  プロフィール          → /users/{handle}   （ハンドルがなければ「ハンドルを設定」→ /settings/profile）
  プロフィールを編集    → /settings/profile
  Namespace の管理      → /settings/namespaces
  ───
  自分の提出            → /me/submissions
  自分の問題            → /me/problems
  ───
  ログアウト（赤字）
```

- 「自分のライブラリ」はプロフィールの Libraries タブと同じ内容になるので、メニューから外す。
- `role="menu"` と `role="menuitem"` は外す。開閉ボタンの `aria-expanded` と、普通のリンクの一覧にする（#55 のユーザーメニュー部分）。

## 権限

| 対象 | 本人 | 他人・未ログイン |
| --- | --- | --- |
| ハンドル・表示名・外部リンク | 閲覧と編集（ハンドルは変更不可） | 閲覧 |
| Libraries の一覧と件数 | public と private | public のみ |
| Problems の一覧と件数 | public / private / group | public のみ |
| メールアドレス・Google の名前 | 出さない | 出さない |

- 他人が private な Library・Problem を直接開いたときは、これまでどおり 404 を返す。

## API（`/api/v1` 以下）

| メソッド | パス | 認証 | 内容 |
| --- | --- | --- | --- |
| GET | `/me` | 必須 | `user_id`, `role`, `handle`, `display_name`（ハンドルがなければ後ろの 2 つは `null`） |
| POST | `/me/profile` | 必須 | ハンドルと表示名を作る。ハンドルが既にあれば 409 `HANDLE_ALREADY_SET`、他人が使っていれば 409 `HANDLE_TAKEN` |
| PATCH | `/me/profile` | 必須 | 表示名と外部リンクを変える。ハンドルがなければ 409 `HANDLE_REQUIRED` |
| GET | `/users/{handle}` | 不要 | プロフィール（`library_count`, `problem_count` を含む） |
| GET | `/users/{handle}/libraries` | 不要 | Namespace をまたぐ Library の一覧。`namespace`, `limit`, `offset` |
| GET | `/users/{handle}/problems` | 不要 | そのユーザーが作った Problem の一覧。`limit`, `offset` |

- Namespace と Library の API は [ライブラリ共有 仕様](library-sharing.md#api) を参照。
- Problem の `created_by` と最近の提出の `submitter` はハンドルで返す（ハンドルのないユーザーなら `null`）。
- ページングは既存と同じ `limit`（1〜100、既定 20）と `offset`（0 以上）。

## 実装の順序

| PR | 内容 |
| --- | --- |
| PR 1 | `user_profiles` とハンドル、初回設定、複数の Namespace、`/users/{handle}/...` への URL と API の移行、`created_by` / `owner` / `submitter` のハンドル化 |
| PR 2 | プロフィールページ（Overview / Libraries / Problems のタブ）、`/settings/profile` の表示名と外部リンク |
| PR 3 | ユーザーメニュー（新しい項目、#55 のユーザーメニュー部分） |
| 以降 | アバターのアップロード |

- フロントを書く前に `frontend/node_modules/next/dist/docs/` で、`searchParams` と、Server Component からの `redirect` / `notFound` の Next.js 16 での扱いを確認する。

## 実装前に確認すること

- TopCoder のハンドルに使える文字と長さ。
- テストケースを更新したときに `problems.updated_at` が更新されるか。更新されないなら、Problems の「最後に更新された順」に含めるかを決める。

## 対象外

ハンドルと Namespace の slug の変更（とそのリダイレクト）、Library の別の Namespace への移動、Library の削除、フォロー、アクティビティの表示、プロフィールの自己紹介文。
