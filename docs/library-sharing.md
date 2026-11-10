# ライブラリ共有 仕様

競技プログラミング用ライブラリを Namespace / Library / Library Version として公開する機能の現行仕様です。
HTTP API の詳細は [OpenAPI](../backend/docs/openapi.yaml) を正とします。

初期構想（Test Suite と Judge Run で Library Version を検証する案）は
[docs/archive/library-sharing-judge-requirements.md](archive/library-sharing-judge-requirements.md) に残していますが、現在の要件ではありません。

## 目的

利用者は普段の方法でコードを提出し、自分の Library Version にその提出 URL を貼るだけで、関連する提出を公開できる。Library と Problem は独立したままにする。

## 用語

| 用語 | 説明 |
| --- | --- |
| Namespace | ユーザーごとに 1 つ持つ公開名前空間。slug は 2〜32 文字の英小文字・数字・単一ハイフン |
| Library | Namespace 配下のライブラリ。slug は 2〜48 文字（同じ規則）。公開範囲は `public` / `private` |
| Library Version | Library のソース（Python 1 ファイル）の不変スナップショット。番号は DB が 1 から採番する |
| 提出 URL | Version ごとに最大 1 件の、所有者が自己申告する「関連する提出」の URL |

## 認可

所有者は Namespace の `owner_user_id` で判定する。ロール（admin など）による例外は無い。

| 操作 | 所有者 | 他人（public Library） | 他人（private Library） | 未ログイン |
| --- | --- | --- | --- | --- |
| Library / Version / 提出 URL の閲覧 | 可 | 可 | 404 | public のみ可 |
| Library の作成・更新、Version の作成、提出 URL の設定・削除 | 可 | 403 | 404 | 401 |

- private Library は所有者以外には存在しないものとして扱う（404）。
- ソースを読む SQL は同じ文の中で現在の公開範囲を確認し、提出 URL を更新する SQL は同じ文の中で所有者を確認する。確認と読み書きの間に公開範囲が切り替わっても漏えいしない。

## Library Version

- ソースは空白のみ不可・NUL 不可・100,000 バイト以内。保存時に前後の空白は除去しない。
- `content_sha256`（ソースの UTF-8 バイト列の SHA-256）と `byte_size` は保存時に計算し、DB の CHECK 制約でも検証する。
- Version は不変。DB トリガーが提出 URL の 2 列以外の更新と、行の削除を拒否する。API にも更新・削除は無い（405）。

## 提出 URL

- `library_versions` の `submission_url`（NULL 可）と `submission_url_updated_at`（URL と同時に設定・削除）に保存する。別テーブルは作らない。
- 絶対 HTTP(S) URL でホストがあり、制御文字・空白を含まず、2048 バイト以内。前後の空白を除去し、スキームを小文字にして保存する。違反は 400 `INVALID_SUBMISSION_URL`。
- URL は取得・解析・判定しない。提出者・判定結果・使用した Library Version の真正性は保証しない。画面では「関連する提出」と表示し、AC などの検証済みバッジは出さない。
- 設定しても Submission / Outbox / S3 オブジェクト / Judge 実行は作られない。既存の Submission API と Judge は変更しない。

## API（`/api/v1` 以下）

| メソッド | パス | 認証 |
| --- | --- | --- |
| POST | `/namespaces` | 必須 |
| GET | `/me/namespace` | 必須 |
| GET | `/namespaces/{namespace_slug}` | 不要 |
| POST / GET | `/namespaces/{namespace_slug}/libraries` | POST のみ必須 |
| GET / PATCH | `/namespaces/{namespace_slug}/libraries/{library_slug}` | PATCH のみ必須 |
| POST / GET | `.../libraries/{library_slug}/versions` | POST のみ必須 |
| GET | `.../versions/{version_number}` | 不要 |
| PUT / DELETE | `.../versions/{version_number}/submission-url` | 必須（所有者のみ） |

- `PUT` は `{ "submission_url": "https://..." }` で設定・置き換えし、Version を 200 で返す。`DELETE` は 204 を返し、未設定でも成功する。
- Version の取得・一覧は `submission_url` と `submission_url_updated_at` を含む（未設定なら `null`）。一覧には `source_code` を含めない。
- 一覧（Library / Version）は `limit`（1〜100、既定 20）と `offset`（0 以上）でページングする。Library は作成日時の新しい順、Version は番号の大きい順。

## 受け入れ確認

- 所有者が Version 作成後に提出 URL を設定・変更・削除でき、Version の取得・一覧で読める。
- 所有者以外は public Library でも提出 URL を変更できない。private Library の URL は非所有者から読めず、公開範囲の切り替えに追従する。
- 設定時に Submission / Outbox / S3 オブジェクトが作られず、Version のソースは変わらない。
- 不正 URL は 400、未ログインでの設定は 401 になる。
- 既存の Judge / Submission のテストが引き続き通る。

## 対象外

fork、draft / publish / yank、ライセンス、API Token、Change Request、検索、複数ファイル・複数言語の Version。
