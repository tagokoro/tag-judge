# Tag Judge フロントエンド（Library Sharing）

Next.js（App Router）+ TypeScript で実装したライブラリ共有 MVP の画面です。
`/api/:path*` rewrite（`next.config.ts`）経由で Go API を呼びます。
単一 HTML や Go による静的配信は使わず、Go API（`:8080`）と Next.js（`:3000`）は別プロセスです。

API 契約は `backend/docs/openapi.yaml`、要件は
`docs/library-sharing.md` に従います。

## ページ構成

すべてのページの上部に共通ヘッダーがあります。左にライブラリ一覧・問題一覧・提出一覧へのリンク、右にログイン
ボタン（ログイン中はユーザー名）を置きます。ユーザー名をクリックすると、自分のライブラリ・自分の提出・
自分の問題とログアウトのメニューが開きます。dev 認証のメニューは `next dev` のときだけ表示されます。

| パス | 内容 | 主な API（すべて同一オリジンの `/api/v1/*`） |
| --- | --- | --- |
| `/` | ライブラリ一覧（全 Namespace の公開 Library） | `GET /libraries` |
| `/problems` | 問題一覧（公開の問題） | `GET /problems` |
| `/submissions` | 提出一覧（公開の問題への最近の提出。ソースは出さない） | `GET /submissions/recent` |
| `/me` | 自分のライブラリ。Namespace があればそのページへ移動し、無ければ作成フォーム | `GET /me/namespace`、`POST /namespaces` |
| `/me/submissions` | 自分の提出 | `GET /submissions` |
| `/me/problems` | 自分が作った問題（非公開を含む） | `GET /me/problems` |
| `/n/[namespace]` | Namespace の Library 一覧（閲覧のみ） | `GET /namespaces/{ns}`、`GET .../libraries` |
| `/n/[namespace]/[library]` | Library の説明と、最新のコード・提出 URL（閲覧のみ） | `GET .../libraries/{lib}`、`GET .../versions`、`GET .../versions/{n}` |
| `/libraries/new` | Library の作成（自分の Namespace に作る） | `GET /me/namespace`、`POST .../libraries` |
| `/n/[namespace]/[library]/edit` | Library の設定（名前・説明・公開範囲） | `PATCH .../libraries/{lib}` |
| `/n/[namespace]/[library]/code/edit` | コードの更新（最新のコードを編集して保存） | `POST .../versions` |
| `/n/[namespace]/[library]/submission-url/edit` | 最新のコードの提出 URL の設定・削除 | `PUT` / `DELETE .../versions/{n}/submission-url` |

- Namespace のページは `/n/` の下に置き、Namespace の slug が `/me` などのアプリのパスや `/api` と衝突しないようにしています。
- 閲覧ページと編集ページは分けています。所有者が自分のページを見ても閲覧ページにはフォームを置かず、
  「設定を編集」「コードを更新」「提出 URL を編集」などのボタンから編集ページへ移ります。
- 編集ボタンと編集ページのフォームは、Namespace の `owner_user_id` と現在の actor が一致するときだけ表示します。
  表示は補助で、認可は API が行います（所有者以外は 403、private は 404）。
- 提出 URL は 1 件だけで、Library 所有者の**自己申告**です。URL の取得・内容解析・
  判定は行わず、検証済み（AC など）バッジやステータス・スコアも表示しません。設定しても
  Submission / Outbox / S3 オブジェクト / Judge 実行は作られません。
- 画面には Version（版）の概念を出しません。常に最新のコードだけを表示します。
  - API 上はコードを保存するたびに新しい Version が作られ、古い Version はサーバーに残ります（画面からは見えません）。
  - 提出 URL は Version ごとに持つので、コードを更新すると提出 URL は空になります。
  - 内容を変えずに保存すると提出 URL が消えるだけなので、保存ボタンは押せないようにしています。

## 認証とセキュリティ

- **cookie セッションが最優先**。マウント時に dev ヘッダーを付けずに `GET /me` を呼び、成功すれば
  cookie セッションを使い dev 認証を無効化します。Google ログインは同一オリジンの
  `GET /api/v1/auth/google/login` への `<a>` リンク（302 をブラウザが追跡）です。
- **dev ヘッダー認証は明示的なオプトイン**。ヘッダーの「dev 認証」メニュー（`next dev` のときだけ表示）で
  有効にしたとき、かつ cookie セッションが無いときだけ `X-User-ID` / `X-User-Role` を送ります
  （サーバー側で `DEV_HEADER_AUTH=true` が必要）。設定はページ遷移では保たれますが、再読み込みで解除されます。
- すべてのリクエストは `credentials: "same-origin"` で `/api/v1/*` のみを呼びます。
  `API_SERVER_ORIGIN` はサーバー側 rewrite だけで使い、ブラウザには公開しません（`NEXT_PUBLIC_` 不使用）。
- **private ソース / 提出 URL の規則を維持**。private Library の `source_code` と提出 URL は
  API の認可（所有者）に従って表示されます。公開 Library のコードは匿名で読めます。
- **HTML は描画しない**。`innerHTML` / `dangerouslySetInnerHTML` は使いません。
  - Library の説明は Markdown と TeX の数式（`$...$` / `$$...$$`）として描画します
    （react-markdown + remark-gfm + remark-math + rehype-katex）。説明に書かれた生の HTML は描画されず、
    リンクの URL は react-markdown の安全な URL フィルタを通ります。
  - コードは CodeMirror（Python のシンタックスハイライト）でテキストとして表示・編集します。
  - 提出 URL は http / https の形式を確認できたものだけリンクにします。クリックすると新しいタブで開き、
    `rel="noopener noreferrer nofollow"` を付けます。自動取得・自動遷移はしません。

## データの読み込みと actor の切り替え

- **public なページはサーバーで描画します**（Server Component）。`/`、`/problems`、`/submissions`、
  `/n/[namespace]`、`/n/[namespace]/[library]` は `lib/serverApi.ts` で Go API から直接データを取得し、
  中身の入った HTML とページごとのタイトルを返します。
  - 取得には訪問者の cookie を付けるので、ログイン中の所有者には private の Library も最初から表示されます。
  - Go API の場所は `lib/apiOrigin.ts`（`API_SERVER_ORIGIN`）で、rewrite と共通です。
  - dev 認証のヘッダーはブラウザにしかないため、dev 認証が有効なときはブラウザ側で取り直します。
    開発ビルドでは、サーバーで 404 だった Library もブラウザ側で取り直します（dev ユーザーが所有者の場合があるため）。
  - 所有者向けのボタン、編集ページ、自分の一覧はブラウザ側で描画します。
- **見つからないページは 404 です**（`app/not-found.tsx`）。存在しない Namespace・Library、
  他人の private Library、所有者以外が開いた編集ページは、すべて同じ「ページが見つかりません」になり、
  何が存在するかを区別できません。描画中のエラーは `app/error.tsx` で受け止めます。
  - 編集ページは所有者の判定をブラウザで行うため、画面は 404 でも HTTP ステータスは 200 です。
- ログイン状態は `components/AuthProvider.tsx` がルートレイアウトで 1 つだけ持ちます。誰が actor かの判定
  （cookie 優先、dev 認証、所有者判定）は React 非依存の `lib/session.ts` にあります。
- ページは `ActorScope` の中で actor ごとに再マウントされます。セッションの確認中もページは表示し、
  ブラウザ側の読み込みだけが確認を待ちます。ログイン・ログアウトや dev ユーザーの切り替えで
  前の actor 向けに読み込んだデータ（private のソースを含む）は破棄され、実行中のリクエストも中止されます。
- ブラウザ側の読み込みは `components/useResource.ts` で行います（サーバーで取得済みのデータは `initial` で受け取ります）。結果は「どの actor が、何を」要求したかの
  キーと一緒に保持し、キーが変わった古い応答は表示しません。対象が変わると前のリクエストは中止されます。

## 必要なもの・起動

- **Node.js 20.9.0 以上** / **npm 10 以上**（`package.json` の `engines`）。
- 依存は厳密固定（exact version）+ `package-lock.json`（lockfileVersion 3）。`npm ci` を使ってください。

```sh
cd frontend
npm ci
npm run dev            # http://localhost:3000/
```

Go API は別ターミナルで `make run-api`（`:8080`）。ブラウザは `:3000` の `/api/:path*` だけを呼び、
`next.config.ts` の rewrite がサーバー側で Go API へ転送します。OAuth コールバックも同じ rewrite を
通るため、`.env` には `GOOGLE_OAUTH_REDIRECT_URL=http://localhost:3000/api/v1/auth/google/callback` と
`AUTH_SUCCESS_REDIRECT_URL=http://localhost:3000/` を設定します（ルートの `.env.example` 参照）。

## スクリプト

| コマンド | 内容 |
| --- | --- |
| `npm run dev` / `build` / `start` | 開発 / 本番ビルド / ビルド済み起動 |
| `npm run lint` | ESLint（`eslint-config-next`） |
| `npm run typecheck` | `next typegen && tsc --noEmit` |
| `npm test` | `node:test` によるユニットテスト |
| `npm run test:watch` | テストの watch 実行 |

ルートの Makefile から `make frontend-check`（lint / typecheck / build）も実行できます。

## テスト

追加のテスト依存は入れていません。Node.js 組み込みの `node:test` + TypeScript 型ストリップ
（Node 22.18+ / 24）で `lib/*.test.mjs` を実行します。対象は「意味のある」部分に絞っています。

- `lib/api.test.mjs` — `apiRequest` の `{data}` アンラップ、same-origin credentials、`/api/v1` 前缀、
  dev ヘッダーのオプトイン、JSON ボディ、`ApiRequestError`、`204`（logout）、
  `probeCookieSession` が dev ヘッダーを送らないこと、提出 URL の PUT / DELETE の
  パス・メソッド・ボディ、
  既存の Problems / Namespace / Library / Version 契約、slug の URL エンコード、
  `withSignal` による `AbortSignal` の伝播。
- `lib/submissionUrl.test.mjs` — 提出 URL の形式検証（絶対 HTTP(S)・ホスト・2048 バイト・
  制御文字・IDN/IPv6）。
- `lib/session.test.mjs` — cookie セッションが dev 認証より優先されること、dev ヘッダーはオプトイン時だけ
  送ること、actor ごとの識別子、所有者判定（大文字小文字を区別しない）。
- `lib/routes.test.mjs` — ページパスの組み立て（`/n/` 配下・slug のエンコード）と編集ページのパス。

## 手動 E2E の目安

`DEV_HEADER_AUTH=true` のローカルで、seed（author `...0001` / admin `...0002`）を使います。
ヘッダーの dev 認証を有効化 → ユーザーメニューの「自分のライブラリ」で Namespace 作成 → private Library 作成 → seed ユーザーを切り替えて
404 になることを確認 → 戻して public 化 → コードを登録 → 提出 URL を設定 → 別ユーザーでは編集フォームが
出ないこと、ログアウト状態でも public のコードと URL が読めることを確認します。

## 制限（設計どおりの MVP 範囲）

- Library のコードは Python ソース 1 本（複数ファイル・複数言語は対象外）。
- fork / draft / publish / yank / ライセンス / API Token / Change Request / 検索は実装しません。
- 提出 URL は検証・判定されず、Submission / Outbox / S3 / Judge は作られません。
  既存の Submission API / Judge は変更していません。
- ブラウザ自動テスト（E2E）は含みません。UI の操作確認は上記の手動手順で行います。
- データ取得はすべてブラウザ側（クライアントコンポーネント）で行い、サーバー側でのデータ取得は使っていません。
