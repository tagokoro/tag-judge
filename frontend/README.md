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
| `/n/[namespace]` | Namespace の Library 一覧。所有者には Library の作成フォーム | `GET /namespaces/{ns}`、`GET .../libraries`、`POST .../libraries` |
| `/n/[namespace]/[library]` | Library の詳細と Version 一覧。所有者には Version の追加と Library の設定 | `GET .../libraries/{lib}`、`GET .../versions`、`POST .../versions`、`PATCH .../libraries/{lib}` |
| `/n/[namespace]/[library]/v/[version]` | Version のソースと提出 URL。所有者には提出 URL の設定・削除 | `GET .../versions/{n}`、`PUT` / `DELETE .../versions/{n}/submission-url` |

- Namespace のページは `/n/` の下に置き、Namespace の slug が `/me` などのアプリのパスや `/api` と衝突しないようにしています。
- 所有者向けのフォームは、Namespace の `owner_user_id` と現在の actor が一致するときだけ表示します。表示は補助で、認可は API が行います（所有者以外は 403、private は 404）。
- 各 Version の「関連する提出 URL」は 1 件だけで、Library 所有者の**自己申告**です。URL の取得・内容解析・
  判定は行わず、検証済み（AC など）バッジやステータス・スコアも表示しません。設定しても
  Submission / Outbox / S3 オブジェクト / Judge 実行は作られません。
- Version は不変です。更新・削除の UI はありません（API も `405`）。

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
  API の認可（所有者）に従って表示されます。公開 Library の Version は匿名で読めます。
- **テキスト表示のみ**。Library / Version のソースと提出 URL は React のテキストノードと
  `<pre>{...}</pre>` で描画し、`innerHTML` / `dangerouslySetInnerHTML` は一切使いません。
  提出 URL はリンクにせずテキストとして表示し、自動取得・自動遷移もしません。

## データの読み込みと actor の切り替え

- ログイン状態は `components/AuthProvider.tsx` がルートレイアウトで 1 つだけ持ちます。誰が actor かの判定
  （cookie 優先、dev 認証、所有者判定）は React 非依存の `lib/session.ts` にあります。
- ページは `ActorScope` の中で actor ごとに再マウントされます。ログイン・ログアウトや dev ユーザーの切り替えで
  前の actor 向けに読み込んだデータ（private のソースを含む）は破棄され、実行中のリクエストも中止されます。
- 各ページの読み込みは `components/useResource.ts` で行います。結果は「どの actor が、何を」要求したかの
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
- `lib/routes.test.mjs` — ページパスの組み立て（`/n/` 配下・slug のエンコード）と Version 番号の解釈。

## 手動 E2E の目安

`DEV_HEADER_AUTH=true` のローカルで、seed（author `...0001` / admin `...0002`）を使います。
ヘッダーの dev 認証を有効化 → ユーザーメニューの「自分のライブラリ」で Namespace 作成 → private Library 作成 → seed ユーザーを切り替えて
404 になることを確認 → 戻して public 化 → Version 作成 → 提出 URL を設定 → 別ユーザーでは編集フォームが
出ないこと、ログアウト状態でも public の Version と URL が読めることを確認します。

## 制限（設計どおりの MVP 範囲）

- Library Version は Python ソース 1 本（複数ファイル・複数言語は対象外）。
- fork / draft / publish / yank / ライセンス / API Token / Change Request / 検索は実装しません。
- 提出 URL は検証・判定されず、Submission / Outbox / S3 / Judge は作られません。
  既存の Submission API / Judge は変更していません。
- ブラウザ自動テスト（E2E）は含みません。UI の操作確認は上記の手動手順で行います。
- データ取得はすべてブラウザ側（クライアントコンポーネント）で行い、サーバー側でのデータ取得は使っていません。
