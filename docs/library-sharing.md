# ライブラリ共有 仕様

競技プログラミング用ライブラリを Namespace / Library / Library Version として公開する機能の現行仕様です。
HTTP API の詳細は [OpenAPI](../backend/docs/openapi.yaml) を正とします。ハンドルとプロフィールは [ユーザープロフィール 仕様](user-profile.md) を参照してください。

初期構想（Test Suite と Judge Run で Library Version を検証する案）は
[docs/archive/library-sharing-judge-requirements.md](archive/library-sharing-judge-requirements.md) に残していますが、現在の要件ではありません。

## 目的

利用者は普段の方法でコードを提出し、自分の Library Version にその提出 URL を貼るだけで、関連する提出を公開できる。Library と Problem は独立したままにする。
Library は分野ごとの Namespace（例：`graph`、`data-structure`、`math`）に分けて置ける。

## 用語

| 用語 | 説明 |
| --- | --- |
| ハンドル | ユーザーの公開 ID。全体で一意（[ユーザープロフィール 仕様](user-profile.md#用語)） |
| Namespace | ユーザーが Library を分けて置く入れ物。1 ユーザーが最大 20 個持てる。slug は 2〜32 文字の英小文字・数字・単一ハイフンで、**同じユーザーの中で一意**（他のユーザーとは同じ slug を使える）。予約語は使えない |
| Library | Namespace 配下のライブラリ。slug は 2〜48 文字（同じ規則）で、Namespace の中で一意。公開範囲は `public` / `private` |
| Library Version | Library のソース（Python 1 ファイル）の不変スナップショット。番号は DB が 1 から採番する |
| 提出 URL | Version ごとに最大 1 件の、所有者が自己申告する「関連する提出」の URL |

## Namespace

- **作成：** ハンドルを持つユーザーだけが作れる（ないときは 409 `HANDLE_REQUIRED`）。slug・名前（`display_name`、1〜64 文字）・説明（`description`、500 文字以内、省略時は空）を指定する。
  - 同じユーザーの中で slug が重複したら 409 `NAMESPACE_SLUG_TAKEN`。
  - 21 個目は 409 `NAMESPACE_LIMIT`。上限の判定と作成は、`user_profiles` の行をロックしたトランザクションの中で行い、同時に作成しても上限を超えないようにする。
  - slug の予約語は `edit`、`new`、`settings`、`libraries`、`problems`、`submissions`。将来 `/users/{handle}/` の下に固定のページを足せるようにするため。違反は 400 `INVALID_SLUG`。
- **変更：** 名前と説明は所有者が変更できる。slug は変更できない（URL が変わるため）。
- **削除：** 所有者が、Library が 1 つもないときだけ削除できる。Library があれば 409 `NAMESPACE_NOT_EMPTY`。判定は削除する SQL の同じ文の中で行う（`libraries.namespace_id` の外部キーも最後の守りになる）。Library の削除と移動は対象外。
- **公開範囲：** Namespace には公開範囲の列を持たせない。所有者以外には、**public な Library を 1 つ以上含む Namespace だけ** が見える。それ以外の Namespace（空、または private な Library だけ）は、所有者以外には 404 にし、一覧にも出さない。判定は読む SQL の同じ文の中で `EXISTS` を使って行う。

## 認可

所有者は Namespace の `owner_user_id` で判定する。ロール（admin など）による例外は無い。API のレスポンスでは、所有者を UUID ではなくハンドル（`owner`）で返す。

| 操作 | 所有者 | 他人（public Library） | 他人（private Library） | 未ログイン |
| --- | --- | --- | --- | --- |
| Library / Version / 提出 URL の閲覧 | 可 | 可 | 404 | public のみ可 |
| Library の作成・更新、Version の作成、提出 URL の設定・削除 | 可 | 403 | 404 | 401 |

| 操作 | 所有者 | 他人（public な Library を含む Namespace） | 他人（それ以外の Namespace） | 未ログイン |
| --- | --- | --- | --- | --- |
| Namespace の閲覧 | 可 | 可 | 404 | 他人と同じ |
| Namespace の変更・削除 | 可 | 403 | 404 | 401 |
| Namespace への Library の作成 | 可 | 403 | 404 | 401 |

- private Library は所有者以外には存在しないものとして扱う（404）。
- ソースを読む SQL は同じ文の中で現在の公開範囲を確認し、提出 URL を更新する SQL は同じ文の中で所有者を確認する。確認と読み書きの間に公開範囲が切り替わっても漏えいしない。

## Library

- **最後の更新日時：** `libraries.updated_at` は、Library の更新（名前・説明・公開範囲）と **Version の作成** で更新する。Version の作成では、Version の INSERT と同じトランザクションで更新する。提出 URL の設定・削除では更新しない（コードは変わらないため）。
- 一覧は、最後に更新された順（`updated_at DESC, slug ASC`）に並べる。ただし全体の公開一覧 `GET /libraries` だけは、今までどおり作成日時の新しい順にする。

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
| `/users/{handle}/{namespace}` | Namespace のページ。名前・`@handle / slug`・説明・Libraries のカード。所有者には「編集」（→ `/settings/namespaces/{namespace}`）と「Library を作成」を出す |
| `/users/{handle}/{namespace}/{library}` | Library のページ |
| `/users/{handle}/{namespace}/{library}/edit` | Library の編集 |
| `/users/{handle}/{namespace}/{library}/code/edit` | コードの差し替え |
| `/users/{handle}/{namespace}/{library}/submission-url/edit` | 提出 URL の設定・削除 |
| `/libraries/new?namespace={slug}` | Library の作成。作成先の Namespace を select で選ぶ |
| `/settings/namespaces`、`/settings/namespaces/{namespace}` | Namespace の管理（[ユーザープロフィール 仕様](user-profile.md#画面と-url)） |

- 旧 URL の `/n/...` は削除する（未リリースなのでリダイレクトは用意しない）。
- **Library の作成ページ：** 作成先の初期値は、`?namespace=` で指定された Namespace、次に Namespace が 1 つだけならそれ、の順で決める。Namespace が 0 個なら、作成フォームの代わりに `/settings/namespaces` への案内を出す。ハンドルがなければ `/settings/profile` への案内を出す。
- パンくずリストは `@handle` → Namespace の slug → Library の slug の順。`@handle` は `/users/{handle}` へのリンク。

## API

すべて `/api/v1` 以下。

| メソッド | パス | 認証 |
| --- | --- | --- |
| GET | `/libraries`（全ユーザーの public Library） | 不要 |
| GET / POST | `/me/namespaces`（自分の Namespace の一覧・作成） | 必須 |
| GET | `/users/{handle}/namespaces` | 不要 |
| GET / PATCH / DELETE | `/users/{handle}/namespaces/{namespace_slug}` | PATCH と DELETE は必須（所有者のみ） |
| POST / GET | `/users/{handle}/namespaces/{namespace_slug}/libraries` | POST のみ必須 |
| GET / PATCH | `.../libraries/{library_slug}` | PATCH のみ必須 |
| POST / GET | `.../libraries/{library_slug}/versions` | POST のみ必須 |
| GET | `.../versions/{version_number}` | 不要 |
| PUT / DELETE | `.../versions/{version_number}/submission-url` | 必須（所有者のみ） |

- 旧 `/namespaces`、`/me/namespace`、`/namespaces/{namespace_slug}/...` は削除する。
- `PUT` は `{ "submission_url": "https://..." }` で設定・置き換えし、Version を 200 で返す。`DELETE` は 204 を返し、未設定でも成功する。
- Version の取得・一覧は `submission_url` と `submission_url_updated_at` を含む（未設定なら `null`）。一覧には `source_code` を含めない。
- 一覧（Namespace / Library / Version）は `limit`（1〜100、既定 20）と `offset`（0 以上）でページングする。Namespace は slug の昇順、Library は「Library」の節の順、Version は番号の大きい順。
- Namespace と Library のレスポンスは、所有者のハンドルを `owner` で返す。`owner_user_id` は返さない。

## マイグレーション

`000010` で次のことを行う（未リリースなので既存データは残さない）。

1. `library_versions`、`libraries`、`namespaces` を `TRUNCATE` する。
2. `user_profiles` を作る（[ユーザープロフィール 仕様](user-profile.md#データモデル)）。
3. `namespaces` から `UNIQUE (owner_user_id)` と `UNIQUE (slug)` を外し、`UNIQUE (owner_user_id, slug)`、予約語の CHECK、`description` 列（`text NOT NULL DEFAULT ''`、500 文字以内）を足す。`owner_user_id` の参照先を `user_profiles (user_id)` に変える。
4. `libraries (namespace_id, updated_at DESC, slug ASC)` のインデックスを足す。

down は逆の順に戻す。1 ユーザー 1 Namespace の制約に戻すため、down でも Namespace 関連のデータを `TRUNCATE` する。

## 受け入れ確認

- 1 ユーザーが複数の Namespace を作れ、別々のユーザーが同じ slug（例：`graph`）の Namespace を作れる。同じユーザーの中での重複と 21 個目は 409 になる。
- 中身のある Namespace は削除できず（409）、空の Namespace は削除できる。
- private な Library しかない Namespace は、所有者以外には 404 になり、一覧にも出ない。
- Version を作ると Library の `updated_at` が更新され、提出 URL の設定では更新されない。
- 所有者が Version 作成後に提出 URL を設定・変更・削除でき、Version の取得・一覧で読める。
- 所有者以外は public Library でも提出 URL を変更できない。private Library の URL は非所有者から読めず、公開範囲の切り替えに追従する。
- 設定時に Submission / Outbox / S3 オブジェクトが作られず、Version のソースは変わらない。
- 不正 URL は 400、未ログインでの設定は 401 になる。
- Namespace・Library・Problem・最近の提出・プロフィールの API のレスポンスに、`owner_user_id` などのユーザーの UUID が含まれない（Group の API は対象外）。
- 既存の Judge / Submission のテストが引き続き通る。

## 対象外

fork、draft / publish / yank、ライセンス、API Token、Change Request、検索、複数ファイル・複数言語の Version、Namespace と Library の slug の変更、Library の削除と別の Namespace への移動。
