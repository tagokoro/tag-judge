# ライブラリ共有 仕様

競技プログラミング用ライブラリを Library / Library Version として公開し、カテゴリで分けて見せる機能の現行仕様です。
HTTP API の詳細は [OpenAPI](../backend/docs/openapi.yaml) を正とします。ハンドルとプロフィールは [ユーザープロフィール 仕様](user-profile.md) を参照してください。

初期構想（Test Suite と Judge Run で Library Version を検証する案）は
[docs/archive/library-sharing-judge-requirements.md](archive/library-sharing-judge-requirements.md) に残していますが、現在の要件ではありません。

## 目的

利用者は普段の方法でコードを提出し、自分の Library Version にその提出 URL を貼るだけで、関連する提出を公開できる。Library と Problem は独立したままにする。
Library は分野ごとのカテゴリ（例：「グラフ」「データ構造」「数学」）に分けて見せられる。カテゴリはユーザーが管理するものではなく、Library に付ける分類として扱う。

## 用語

| 用語 | 説明 |
| --- | --- |
| ハンドル | ユーザーの唯一の名前。全体で一意（[ユーザープロフィール 仕様](user-profile.md#用語)） |
| ID | カテゴリと Library を識別する、システムが付ける 20 桁の 16 進数のランダムな文字列（例：`70da733124b8e860f0fd`）。全体で一意。変わらない。連番ではないので、件数や作成順は外から分からない |
| カテゴリ | Library の分類。名前は自由（日本語も可）で、同じユーザーの中では重複できない（英字の大文字・小文字は区別しない）。1 ユーザーが最大 20 個持てる |
| Library | どれか 1 つのカテゴリに属するライブラリ。名前は自由で、重複してよい。公開範囲は `public` / `private`。同じユーザーの別のカテゴリに移せる |
| Library Version | Library のソース（Python 1 ファイル）の不変スナップショット。番号は DB が 1 から採番する |
| 提出 URL | Version ごとに最大 1 件の、所有者が自己申告する「関連する提出」の URL |

- ユーザーが考える名前は、ハンドルと、Library とカテゴリの名前だけにする。URL に使う識別子（slug）は入力させない。
- 名前は前後の空白を除いて保存する。カテゴリの名前は 1〜64 文字、Library の名前は 1〜80 文字。

## カテゴリ

カテゴリを作る・消す操作はない。Library に付けるカテゴリ名に合わせて、サーバーが自動で作り、消す。

- **作成：** Library を作るとき、または Library のカテゴリを変えるときに、カテゴリ名を指定する。自分のカテゴリに同じ名前（大文字・小文字は区別しない）があればそれを使い、なければ新しく作る。
  - ハンドルのないユーザーは Library を作れない（409 `HANDLE_REQUIRED`）。
  - 新しいカテゴリが 21 個目になるときは 409 `CATEGORY_LIMIT`。既存のカテゴリを使う場合は上限に関係しない。
  - 「カテゴリを探す → なければ作る → Library を作る（または移す）」は 1 つのトランザクションで行い、その間は `user_profiles` の行をロックする。同時に書き込んでも、同じ名前のカテゴリが 2 つできたり、上限を超えたりしない。Library の作成に失敗したときは、カテゴリも作られない。
- **削除：** Library を移して元のカテゴリに Library が 1 つもなくなったら、同じトランザクションの中でそのカテゴリを削除する。説明も一緒に消える。
- **変更：** 名前と説明は所有者がいつでも変更できる（カテゴリのページの「編集」から）。ID は変わらないので、URL も変わらない。自分の別のカテゴリと同じ名前にすると 409 `CATEGORY_NAME_TAKEN`。
- **公開範囲：** カテゴリには公開範囲の列を持たせない。所有者以外には、**public な Library を 1 つ以上含むカテゴリだけ** が見える。それ以外のカテゴリ（private な Library だけ）は、所有者以外には 404 にし、一覧にも出さない。判定は読む SQL の同じ文の中で `EXISTS` を使って行う。

## 認可

所有者はカテゴリの `owner_user_id` で判定する。ロール（admin など）による例外は無い。API のレスポンスでは、所有者を UUID ではなくハンドル（`owner`）で返す。

| 操作 | 所有者 | 他人（public Library） | 他人（private Library） | 未ログイン |
| --- | --- | --- | --- | --- |
| Library / Version / 提出 URL の閲覧 | 可 | 可 | 404 | public のみ可 |
| Library の更新・カテゴリの変更、Version の作成、提出 URL の設定・削除 | 可 | 403 | 404 | 401 |

| 操作 | 所有者 | 他人（public な Library を含むカテゴリ） | 他人（それ以外のカテゴリ） | 未ログイン |
| --- | --- | --- | --- | --- |
| カテゴリの閲覧 | 可 | 可 | 404 | 他人と同じ |
| カテゴリの名前・説明の変更 | 可 | 403 | 404 | 401 |

- Library の作成は、いつも自分のカテゴリに対して行う（他人のカテゴリには作れない）。
- private Library は所有者以外には存在しないものとして扱う（404）。
- ID の形式が正しくない（20 桁の 16 進数でない）ときも、存在しないときと同じ 404 にする。
- ソースを読む SQL は同じ文の中で現在の公開範囲を確認し、提出 URL を更新する SQL は同じ文の中で所有者を確認する。確認と読み書きの間に公開範囲が切り替わっても漏えいしない。

## Library

- **最後の更新日時：** `libraries.updated_at` は、Library の更新（名前・説明・公開範囲・カテゴリの変更）と **Version の作成** で更新する。Version の作成では、Version の INSERT と同じトランザクションで更新する。提出 URL の設定・削除では更新しない（コードは変わらないため）。
- 一覧は、最後に更新された順（`updated_at DESC, id ASC`）に並べる。ただし全体の公開一覧 `GET /libraries` だけは、今までどおり作成日時の新しい順にする。

## Library Version

- ソースは空白のみ不可・NUL 不可・100,000 バイト以内。保存時に前後の空白は除去しない。
- `content_sha256`（ソースの UTF-8 バイト列の SHA-256）と `byte_size` は保存時に計算し、DB の CHECK 制約でも検証する。
- Version は不変。DB トリガーが提出 URL の 2 列以外の更新と、行の削除を拒否する。API にも更新・削除は無い（405）。

## 提出 URL

- `library_versions` の `submission_url`（NULL 可）と `submission_url_updated_at`（URL と同時に設定・削除）に保存する。別テーブルは作らない。
- 絶対 HTTP(S) URL でホストがあり、制御文字・空白を含まず、2048 バイト以内。前後の空白を除去し、スキームを小文字にして保存する。違反は 400 `INVALID_SUBMISSION_URL`。
- URL は取得・解析・判定しない。提出者・判定結果・使用した Library Version の真正性は保証しない。画面では「関連する提出」と表示し、AC などの検証済みバッジは出さない。
- 設定しても Submission / Outbox / S3 オブジェクト / Judge 実行は作られない。既存の Submission API と Judge は変更しない。

## 画面と URL

| URL | 内容 |
| --- | --- |
| `/users/{handle}/categories/{id}` | カテゴリのページ。名前・`@handle`・説明・Libraries のカード。所有者には「編集」と「Library を作成」を出す |
| `/users/{handle}/categories/{id}/edit` | カテゴリの名前・説明の変更（所有者のみ） |
| `/users/{handle}/libraries/{id}` | Library のページ |
| `/users/{handle}/libraries/{id}/edit` | Library の設定（名前・説明・公開範囲・カテゴリ） |
| `/users/{handle}/libraries/{id}/code/edit` | コードの差し替え |
| `/users/{handle}/libraries/{id}/submission-url/edit` | 提出 URL の設定・削除 |
| `/libraries/new?category={id}` | Library の作成 |

- **カテゴリの入力欄**（Library の作成と設定）：自分の既存のカテゴリを候補から選ぶか、新しい名前を入力する（`<input list>` と `<datalist>`）。既存と一致しない名前のときは「「○○」は新しいカテゴリとして作られます。」と出す。`?category={id}` を付けると、そのカテゴリの名前を最初から入れておく。
- カテゴリを一覧・作成・削除する管理画面は置かない。
- URL のハンドルと、カテゴリ・Library の実際の所有者が違うときは 404 にする（ハンドルは変わらないので、正しい URL は 1 つだけ）。
- Library の URL にはカテゴリを入れない。Library を別のカテゴリに移しても URL は変わらない。
- 旧 URL の `/n/...` は削除する（未リリースなのでリダイレクトは用意しない）。
- ハンドルがなければ、Library の作成ページは `/signup` への案内を出す。
- パンくずリストは `@handle` → カテゴリの名前 → Library の名前の順。

## API

すべて `/api/v1` 以下。カテゴリと Library は ID だけで引ける（ハンドルはパスに入れない）。

| メソッド | パス | 認証 |
| --- | --- | --- |
| GET | `/libraries`（全ユーザーの public Library） | 不要 |
| POST | `/me/libraries`（`{ name, category, description, visibility }` で作成。`category` はカテゴリの名前） | 必須 |
| GET | `/me/categories`（自分のカテゴリの一覧。入力欄の候補に使う） | 必須 |
| GET | `/users/{handle}/categories` | 不要 |
| GET / PATCH | `/categories/{category_id}` | PATCH のみ必須（所有者のみ） |
| GET | `/categories/{category_id}/libraries` | 不要 |
| GET / PATCH | `/libraries/{library_id}` | PATCH のみ必須。`category`（名前）を渡すとカテゴリを変える |
| POST / GET | `/libraries/{library_id}/versions` | POST のみ必須 |
| GET | `/libraries/{library_id}/versions/{version_number}` | 不要 |
| PUT / DELETE | `/libraries/{library_id}/versions/{version_number}/submission-url` | 必須（所有者のみ） |

- カテゴリを作る・削除する API は無い。旧 Namespace の API（`/namespaces/...`、`/me/namespace`）は削除する。
- カテゴリ名が不正なとき、Library の作成・更新では 400 `INVALID_CATEGORY`、カテゴリ自体の名前の変更では 400 `INVALID_NAME`。
- `PUT` は `{ "submission_url": "https://..." }` で設定・置き換えし、Version を 200 で返す。`DELETE` は 204 を返し、未設定でも成功する。
- Version の取得・一覧は `submission_url` と `submission_url_updated_at` を含む（未設定なら `null`）。一覧には `source_code` を含めない。
- 一覧（カテゴリ / Library / Version）は `limit`（1〜100、既定 20）と `offset`（0 以上）でページングする。カテゴリは作成日時の古い順、Library は「Library」の節の順、Version は番号の大きい順。
- カテゴリのレスポンスは `id`・`owner`（ハンドル）・`name`・`description`。Library のレスポンスは `id`・`owner`・`category`（`id` と `name`）・`name`・`description`・`visibility`。内部の UUID や `owner_user_id` は返さない。

## マイグレーション

`000010` で次のことを行う（未リリースなので既存データは残さない）。

1. `library_versions`、`libraries`、`namespaces` を `TRUNCATE` する。
2. `user_profiles` を作る（[ユーザープロフィール 仕様](user-profile.md#データモデル)）。
3. `namespaces` を削除し、`categories` を作る。
   - `public_id`：20 桁の 16 進数。`encode(gen_random_bytes(10), 'hex')` を既定値にし、UNIQUE と形式の CHECK を付ける。
   - `owner_user_id`：`user_profiles (user_id)` を参照する。
   - `name`：前後の空白を除いて 1〜64 文字。`(owner_user_id, lower(name))` の UNIQUE インデックス。
   - `description`：500 文字以内。
4. `libraries` から `slug` とそれに付いた制約を外し、`public_id` を足す（`categories` と同じ作り）。`namespace_id` を `category_id` に改名して `categories (id)` を参照させる。インデックスは `updated_at DESC` の並びに合わせて作り直す。
5. 内部の主キー（UUID）と、それを参照する外部キーはそのまま使う。API には `public_id` だけを `id` として出す。

down は 000009 の状態（1 ユーザー 1 Namespace）に戻す。戻せないデータがあるので、down でも Library 関連のデータを `TRUNCATE` する。

## 受け入れ確認

- Library を作るときに新しいカテゴリ名を書くとカテゴリが作られ、既存の名前（大文字・小文字の違いは無視）を書くとそのカテゴリに入る。別々のユーザーが同じ名前のカテゴリを持てる。
- 新しいカテゴリが 21 個目になる作成・移動は 409 になる。既存のカテゴリへの作成・移動は上限でもできる。
- ハンドルのないユーザーの作成は 409 になり、カテゴリも残らない。
- Library を移して元のカテゴリが空になると、そのカテゴリは削除される。他に Library が残るカテゴリは残る。
- カテゴリと Library の名前を変えても URL（ID）は変わらない。Library を別のカテゴリに移しても URL は変わらない。
- private な Library しかないカテゴリは、所有者以外には 404 になり、一覧にも出ない。
- URL のハンドルと所有者が違うカテゴリ・Library のページは 404 になる。
- Version を作ると Library の `updated_at` が更新され、提出 URL の設定では更新されない。
- 所有者が Version 作成後に提出 URL を設定・変更・削除でき、Version の取得・一覧で読める。
- 所有者以外は public Library でも提出 URL を変更できない。private Library の URL は非所有者から読めず、公開範囲の切り替えに追従する。
- 設定時に Submission / Outbox / S3 オブジェクトが作られず、Version のソースは変わらない。
- 不正 URL は 400、未ログインでの設定は 401 になる。
- カテゴリ・Library・Problem・最近の提出・プロフィールの API のレスポンスに、ユーザーの UUID もカテゴリ・Library の内部 UUID も含まれない（自分の提出と Group の API は対象外）。
- 既存の Judge / Submission のテストが引き続き通る。

## 対象外

fork、draft / publish / yank、ライセンス、API Token、Change Request、検索、複数ファイル・複数言語の Version、Library の削除、カテゴリの手動の作成・削除。
