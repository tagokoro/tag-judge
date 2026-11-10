# Tag Judge フロントエンド（Library Sharing）

Next.js（App Router）+ TypeScript で実装したライブラリ共有 MVP の画面です。
PR3 で用意した環境・`/api/:path*` rewrite の上に、実際に操作できる UI を載せています。
単一 HTML や Go による静的配信は使わず、Go API（`:8080`）と Next.js（`:3000`）は別プロセスです。

API 契約は `backend/docs/openapi.yaml`、提出 URL リンクの設計は
`docs/superpowers/specs/2026-11-10-verification-url-design.md` に従います。

## 実装したフロー

| 画面 | 操作（すべて同一オリジンの `/api/v1/*`） |
| --- | --- |
| 認証 / セッション | `GET /me` で cookie セッションを自動判定、`/auth/google/login` へのログインリンク、`POST /auth/logout`、dev ヘッダー認証 |
| Namespace | `POST /namespaces`、`GET /me/namespace`、`GET /namespaces/{slug}` |
| Library | `POST /namespaces/{slug}/libraries`、`GET .../libraries`、`GET .../libraries/{lib}`、`PATCH .../libraries/{lib}`（public/private） |
| Version | `POST .../versions`、`GET .../versions`、`GET .../versions/{n}`（ソース表示） |
| 関連する提出URL | `POST .../versions/{n}/verifications`（`{ "submission_url": "https://..." }` で登録）、`GET .../versions/{n}/verifications`（一覧）、`GET /library-verifications/{id}`（詳細） |

- 各 Version に貼られた「関連する提出 URL」は提出者の**自己宣言**です。URL の取得・内容解析・
  判定は行わず、提出者・判定結果・使用 Library Version の真正性も保証しません。
  **検証済み（AC など）バッジやステータス・スコア・ケース結果は表示しません。**
- 登録は認証済みユーザーが行います。公開 Library は誰でも、private Library は所有者だけが
  追加できます。閲覧は公開 Library なら匿名でも可能で、private Library のリンクは所有者以外
  404 です（公開範囲の切り替えに追従）。未認証の登録は 401、不正 URL は 400 です。
- URL は絶対 HTTP(S)（ホストあり・前後空白除去・制御文字なし・2048 バイト以内）。
  登録しても Submission / Outbox / S3 オブジェクト / Judge 実行は一切作られません。
  既存の Submission API と Judge のコード・挙動は変更していません。
- Version は不変です。更新・削除の UI はありません（API も `405`）。
- 「ユーザー切替」は dev ヘッダーの値を seed author / admin で入れ替え、現在の Library 詳細を
  再取得します（private は他ユーザーに 404、public 化後は 200 になることを同じページで確認可能）。

## 認証とセキュリティ

- **cookie セッションが最優先**。マウント時に dev ヘッダーを付けずに `GET /me` を呼び、成功すれば
  cookie セッションを使い dev 認証を無効化します。Google ログインは同一オリジンの
  `GET /api/v1/auth/google/login` への `<a>` リンク（302 をブラウザが追跡）です。
- **dev ヘッダー認証は明示的なオプトイン**。チェックボックスを入れたとき、かつ cookie セッションが
  無いときだけ `X-User-ID` / `X-User-Role` を送ります。有効時は「`DEV_HEADER_AUTH=true` のローカル
  専用・本番では使用禁止」の警告を画面に表示します（サーバー側で `DEV_HEADER_AUTH=true` が必要）。
- すべてのリクエストは `credentials: "same-origin"` で `/api/v1/*` のみを呼びます。
  `API_SERVER_ORIGIN` はサーバー側 rewrite だけで使い、ブラウザには公開しません（`NEXT_PUBLIC_` 不使用）。
- **private ソース / リンクの規則を維持**。private Library の `source_code` と提出リンクは
  API の認可（所有者）に従って表示されます。公開リンクは匿名で読めます。
- **テキスト表示のみ**。Library / Version のソースと提出 URL は React のテキストノードと
  `<pre>{...}</pre>` で描画し、`innerHTML` / `dangerouslySetInnerHTML` は一切使いません。
  URL を自動取得・自動遷移させず、JSON レスポンスも `JSON.stringify` を `<pre>` に表示するだけです。

## 認証・利用者変更時のデータ無効化

画面の状態は `lib/workspaceState.ts` の単一 reducer（React 非依存の純関数）に集約しています。
cookie ログイン/ログアウト、dev 認証の ON/OFF、dev ユーザー ID / ロールの変更、ユーザー切替などで
**実効 actor が変わると**、`authGeneration` を進めて次をまとめて行います。

- Namespace / Library 詳細、Library / Version 一覧、Version の `source_code`、提出リンク一覧・
  選択中のリンク詳細、URL 入力など、actor に紐づくデータをすべてクリアし、選択 Version をリセット。
- 選択 Version は Namespace / Library の slug に**バインド**され、slug が変わると
  `effectiveVersion()` が `null` を返すため、別ライブラリの古い Version 番号を黙って使いません。
- 各パネルを `authGeneration` で再マウントし、`useAsyncAction` の `AbortController` が実行中の
  fetch を中止。遅れて届いたレスポンスは `APPLY_DATA` の世代ガードで破棄され、前の actor の
  private データが復活しません。
- private Library 詳細の再取得が 404 になった場合は、古い詳細と Version ソースを明示的にクリア。

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
  `probeCookieSession` が dev ヘッダーを送らないこと、提出リンク用エンドポイントの
  パス・メソッド・ボディ（旧 `problem_id` / `adapter_source` を含まないこと）、
  既存の Problems / Namespace / Library / Version 契約、slug の URL エンコード、
  `withSignal` による `AbortSignal` の伝播。
- `lib/verification.test.mjs` — 提出 URL の形式検証（絶対 HTTP(S)・ホスト・2048 バイト・
  制御文字・IDN/IPv6）、旧 judge/polling ヘルパーが**存在しない**こと、デフォルト値。
- `lib/workspaceState.test.mjs` — 状態無効化とレースの回帰テスト。actor 変更時の全 private データ
  消去・世代インクリメント・Version リセット、`authGeneration` による遅延レスポンスの破棄、
  `CLEAR_LIBRARY_DETAIL`（404）の消去、Version の Namespace / Library バインド、
  同一セッション再確認では消去しないこと、デフォルト値の整合性。

## 手動 E2E の目安

`DEV_HEADER_AUTH=true` のローカルで、seed（author `...0001` / admin `...0002`）を使います。
dev 認証をチェック → Namespace 作成 → private Library 作成 → 別ユーザーで 404 → public 化 →
Version 作成 → 絶対 HTTP(S) URL を貼ってリンク登録 → 一覧・詳細で読めることを確認 →
public/private を切り替えて匿名・非所有者から読める/読めないことを確認します。
Problem 選択・adapter 入力・判定ポーリング・Submission 作成は行いません。

## 制限（設計どおりの MVP 範囲）

- Library Version は Python ソース 1 本（複数ファイル・複数言語は対象外）。
- fork / draft / publish / yank / ライセンス / API Token / Change Request / 検索は実装しません。
- 提出 URL は検証・判定されず、Submission / Outbox / S3 / Judge は作られません。
  既存の Submission API / Judge は変更していません。
- ブラウザ自動テスト（E2E）は含みません。UI の操作確認は上記の手動手順で行います。
- 画面は 1 ページ構成で、ルーティングや SSR データ取得は使っていません（クライアントコンポーネント）。
