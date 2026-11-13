# ユーザープロフィール 仕様

ユーザーのハンドル・外部リンクと、プロフィールページ（Overview / Libraries / Problems）の仕様です。
カテゴリと Library の仕様は [ライブラリ共有 仕様](library-sharing.md)、HTTP API の詳細は [OpenAPI](../backend/docs/openapi.yaml) を正とします。

## 目的

- ユーザーを UUID ではなくハンドルで識別し、`/users/{handle}` でそのユーザーの Library と Problem をまとめて見られるようにする。
- Library を分野ごとのカテゴリ（例：「グラフ」「数学」）に分けて見せられるようにする。カテゴリは Library に付ける分類で、ユーザーが管理するものにはしない。
- ユーザーが考える名前を減らす。ユーザーの名前はハンドル 1 つだけで、Library とカテゴリは名前を自由に付けるだけにする（識別子はシステムが付ける。Qiita の `/{ユーザー名}/items/{ID}` と同じ形）。
- ヘッダーのユーザーメニューに、プロフィールと設定への入口を置く。

## 用語

| 用語 | 説明 |
| --- | --- |
| ハンドル | ユーザーの唯一の名前で、公開 ID。2〜16 文字の英小文字・数字・単一ハイフン（`^[a-z0-9]+(?:-[a-z0-9]+)*$`）。全体で一意。一度決めたら変更できない。表示名は別に持たない |
| プロフィール | ハンドルと外部リンク。だれでも閲覧できる |

- Google アカウントの名前（`users.name`）とメールアドレスは、どの API・画面にも出さない。
- カテゴリ・Library・Problem・最近の提出・プロフィールの API は、ユーザーを UUID ではなくハンドルで表す。`GET /me` の `user_id`（自分自身の ID）はそのまま残す。自分の提出の API（`user_id`）と Group の API（`created_by` など）は今回は変えない。

## データモデル

```sql
CREATE TABLE user_profiles (
    user_id uuid PRIMARY KEY REFERENCES users (id),
    handle text NOT NULL,            -- UNIQUE、ハンドルの規則を CHECK
    atcoder_username text,           -- 外部リンク（第 2 段階で列を足す）。NULL は未設定
    topcoder_username text,
    x_username text,
    github_username text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

- 公開 API の SQL は `user_profiles` を読み、`users` は読まない。メールアドレスが漏れる経路を作らないためである。
- `categories.owner_user_id` は `user_profiles (user_id)` を参照する。ハンドルのないユーザーはカテゴリも Library も持てない。
- マイグレーション `000010` で `user_profiles` を作る。`000011` で Library 関連の既存データを消してから、`namespaces` を `categories` に作り直す。未リリースなので、既存データとの互換性や旧 URL（`/n/...`）のリダイレクトは用意しない。詳しくは [ライブラリ共有 仕様](library-sharing.md#マイグレーション) を参照。
- 開発用の seed（`backend/db/seeds/development.sql`）では、seed ユーザーにハンドル `seed-author` と `seed-admin` を付ける。

## 登録（ハンドルの作成）

ハンドルは、登録専用のページ `/signup` で本人が入力して決める。

1. Google ログインの callback で、ユーザーにハンドルがなければ、`AUTH_SUCCESS_REDIRECT_URL` と同じオリジンの `/signup` に転送する（設定を増やさないため）。ハンドルがあれば、これまでどおり `AUTH_SUCCESS_REDIRECT_URL` に転送する。
2. `/signup` には、ハンドルの入力欄と「登録」ボタンだけを置く。入力欄は空にしておき、候補は入れない（メールアドレスや Google の名前も使わない）。欄の下に規則（2〜16 文字の英小文字・数字・ハイフン）と「ハンドルは公開され、あとから変更できません」を書く。
3. 「登録」で `POST /me/profile` を呼び、成功したら `/users/{handle}` に移る。他のユーザーが使っているハンドルなら、409 `HANDLE_TAKEN` を受けて「このハンドルは使われています」と出す。
4. すでにハンドルがある人が `/signup` を開いたら `/users/{handle}` に、ログインしていない人には Google ログインへの案内を出す。
5. ハンドルを作るまでの間も、閲覧と提出はできる。Library と Problem の作成だけは、API が 409 `HANDLE_REQUIRED` を返す。
6. ハンドルがない間、ユーザーメニューの最初の項目は「ハンドルを登録」（→ `/signup`）にする。

## 画面と URL

| URL | 内容 | 描画 |
| --- | --- | --- |
| `/users/{handle}` | プロフィール + Overview タブ | Server Component |
| `/users/{handle}?tab=libraries` | Libraries タブ。`&category={id}` で絞り込み、`&page={n}` でページ送り | Server Component |
| `/users/{handle}?tab=problems` | Problems タブ。`&page={n}` でページ送り | Server Component |
| `/users/{handle}/categories/{id}` | カテゴリのページ（[ライブラリ共有 仕様](library-sharing.md#画面と-url)） | Server Component |
| `/users/{handle}/libraries/{id}` | Library のページ（同上） | Server Component |
| `/signup` | ハンドルの登録（登録専用。ハンドルがない人だけ） | クライアント（ログイン必須） |
| `/settings/profile` | ハンドルの表示、外部リンクの編集（第 2 段階）。ハンドルがなければ `/signup` へ転送 | クライアント（ログイン必須） |
| `/me` | ハンドルがあれば `/users/{handle}` へ、なければ `/signup` へ転送 | クライアント |

- `/me/problems` と `/me/submissions` は今のまま残す。
- 閲覧ページと編集ページは分ける。プロフィールページには、本人にだけ濃い青の「編集」（→ `/settings/profile`）を出す。
- 存在しないハンドルは 404 にする。

### プロフィールページ `/users/{handle}`

```
┌───────────────────────────────────────────────┐
│ (頭文字)  @handle                       [編集] │  ← 編集は本人だけ
│           AtCoder ↗ X ↗ GitHub ↗               │  ← 設定したリンクだけ（第 2 段階）
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
| Libraries | 最後に更新された Library を 6 件。各行にカテゴリの名前を付ける。右上に「すべて見る →」（`?tab=libraries`） |
| Problems | 最後に更新された Problem を 5 件。右上に「すべて見る →」（`?tab=problems`） |

- 中身が 0 件のカードは、空であることを短く書いて表示する（例：「public な Library はまだありません。」）。本人には作成への案内を出す。

#### Libraries タブ

- 上部にカテゴリの絞り込みを横並びのボタンで出す（`すべて` + 閲覧者が見られるカテゴリ）。選択中のボタンに `aria-current="page"` を付ける。
- 一覧は `GET /users/{handle}/libraries?category={id}&limit=20&offset=` を使う。並びは最後に更新された順。
- 閲覧者が見られないカテゴリ（存在しない、他のユーザーのもの、または他人から見て public な Library がない）を `category=` に指定したときは、空の一覧を出す。
- 本人には右上に「Library を作成」（`/libraries/new`、絞り込み中なら `?category={id}` を付ける）と、各行に `public` / `private` のバッジを出す。

#### Problems タブ

- `GET /users/{handle}/problems?limit=20&offset=` を使う。並びは最後に更新された順。
- 本人には `private` / `group` を含めて出し、公開範囲のバッジを付ける。他人には `public` だけを出す。

### 設定ページ `/settings/profile`

- ハンドルがある場合、ハンドルは変更できない値として表示だけする。
- 外部リンク（第 2 段階）は、サービスごとに URL の前半（例：`https://atcoder.jp/users/`）を入力欄の前に出し、完成する URL が分かるようにする。「保存」で `PATCH /me/profile` を呼ぶ。
- フォームを `<form>` にして Enter キーで送信できるようにするのは、全ページまとめて #53 で行う。

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

- 第 1 段階：ハンドルの先頭 1 文字を大文字にして、水色の丸の中に出す（ヘッダーの今の `.user-avatar` と同じ部品を使う）。読み上げではハンドルと重複するので `aria-hidden` にする。
- 第 2 段階（別の PR）：TagJudge に画像をアップロードできるようにする。
  - 受け付ける画像は PNG / JPEG / WebP で、1 MB まで。
  - サーバーで画像として読み込み直し、256×256 の PNG に変換して保存する。EXIF などのメタデータを消し、画像に見せかけた別のファイルも防ぐ。
  - 保存先は今ある S3 / LocalStack の `avatars/` 以下。Go の API から `Content-Type: image/png` と `X-Content-Type-Options: nosniff` を付けて配信する。
  - アップロードしていないユーザーには、第 1 段階と同じく頭文字を出す。

## ユーザーメニュー

ヘッダー右のボタンは、頭文字のアバターだけの丸いボタンにする（ハンドルがなければ「?」）。誰のアカウントかは、ボタンの読み上げ名とツールチップ（「アカウントメニュー（@handle）」）と、開いたメニューの先頭（`@handle` とロール）で分かるようにする。PC のメニューとスマホの `PhoneSheet` の項目は同じにする。

```
  プロフィール          → /users/{handle}   （ハンドルがなければ「ハンドルを登録」→ /signup）
  プロフィールを編集    → /settings/profile
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
| ハンドル・外部リンク | 閲覧と編集（ハンドルは変更不可） | 閲覧 |
| Libraries の一覧と件数 | public と private | public のみ |
| Problems の一覧と件数 | public / private / group | public のみ |
| メールアドレス・Google の名前 | 出さない | 出さない |

- 他人が private な Library・Problem を直接開いたときは、これまでどおり 404 を返す。

## API（`/api/v1` 以下）

| メソッド | パス | 認証 | 内容 |
| --- | --- | --- | --- |
| GET | `/me` | 必須 | `user_id`, `role`, `handle`（ハンドルがなければ `null`） |
| POST | `/me/profile` | 必須 | `{ "handle": "..." }` でハンドルを作る。既にあれば 409 `HANDLE_ALREADY_SET`、他人が使っていれば 409 `HANDLE_TAKEN` |
| PATCH | `/me/profile` | 必須 | 外部リンクを変える（第 2 段階）。ハンドルがなければ 409 `HANDLE_REQUIRED` |
| GET | `/users/{handle}` | 不要 | プロフィール（第 2 段階で `library_count`, `problem_count`, 外部リンクを足す） |
| GET | `/users/{handle}/libraries` | 不要 | カテゴリをまたぐ Library の一覧。`category`（カテゴリの ID）, `limit`, `offset` |
| GET | `/users/{handle}/problems` | 不要 | そのユーザーが作った Problem の一覧。`limit`, `offset` |

- カテゴリと Library の API は [ライブラリ共有 仕様](library-sharing.md#api) を参照。
- Problem の `created_by` と最近の提出の `submitter` はハンドルで返す（ハンドルのないユーザーなら `null`）。
- ページングは既存と同じ `limit`（1〜100、既定 20）と `offset`（0 以上）。

## 実装の順序

| PR | 内容 |
| --- | --- |
| PR 1 | `user_profiles` とハンドル、登録ページ `/signup`、カテゴリ（Library の作成・移動で自動で作り、空になれば消す）、カテゴリと Library のランダムな ID と自由な名前、`/users/{handle}/...` の URL と、ID で引く API への移行、`created_by` / `owner` / `submitter` のハンドル化。`/users/{handle}` は仮に「プロフィール + Categories のカード」とし、`GET /users/{handle}` はハンドルだけを返す |
| PR 2 | プロフィールページ（Overview / Libraries / Problems のタブ）、外部リンク（`PATCH /me/profile`）、`GET /users/{handle}/libraries`・`/problems`、`GET /users/{handle}` の件数と外部リンク。OpenAPI にはこの PR で足す |
| PR 3 | ユーザーメニュー（新しい項目、#55 のユーザーメニュー部分） |
| 以降 | アバターのアップロード |

- フロントを書く前に `frontend/node_modules/next/dist/docs/` で、`searchParams` と、Server Component からの `redirect` / `notFound` の Next.js 16 での扱いを確認する。

## 実装前に確認すること

- TopCoder のハンドルに使える文字と長さ。
- （確認済み）テストケースを更新すると `SetCurrentTestCasesKey` が `problems.updated_at` も更新するので、Problems の「最後に更新された順」に含まれる。

## 対象外

ハンドルの変更（とそのリダイレクト）、Library の削除、フォロー、アクティビティの表示、プロフィールの自己紹介文。
