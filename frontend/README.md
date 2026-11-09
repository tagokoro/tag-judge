# Tag Judge フロントエンド（Library Sharing MVP）

Next.js（App Router）+ TypeScript で実装したライブラリ共有ジャッジ MVP の画面です。
PR3 で用意した環境・`/api/:path*` rewrite の上に、実際に操作できる UI を載せています。
単一 HTML や Go による静的配信は使わず、Go API（`:8080`）と Next.js（`:3000`）は別プロセスです。

API 契約は `backend/docs/openapi.yaml`、機能範囲は
`docs/superpowers/specs/2026-11-08-library-sharing-mvp-design.md` に従います。

## 実装したフロー

| 画面 | 操作（すべて同一オリジンの `/api/v1/*`） |
| --- | --- |
| 認証 / セッション | `GET /me` で cookie セッションを自動判定、`/auth/google/login` へのログインリンク、`POST /auth/logout`、dev ヘッダー認証 |
| Namespace | `POST /namespaces`、`GET /me/namespace`、`GET /namespaces/{slug}` |
| Library | `POST /namespaces/{slug}/libraries`、`GET .../libraries`、`GET .../libraries/{lib}`、`PATCH .../libraries/{lib}`（public/private） |
| Version | `POST .../versions`、`GET .../versions`、`GET .../versions/{n}`（ソース表示） |
| 検証 | `GET /problems` で既存 Problem を選択、`POST .../versions/{n}/verifications`、`GET /library-verifications/{id}` をポーリング、`GET /library-verifications` 一覧 |

- 検証開始後は `GET /api/v1/library-verifications/{id}` を **1 秒間隔・最大 30 回**ポーリングし、
  終端状態（`ac / wa / tle / mle / re / ce / ie`）まで待ちます。ステータス・スコア・
  ケース別結果（`test_case_id` / `status` / `execution_time_ms` / `memory_kb`）を表示します。
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
- **private ソース / adapter の規則を維持**。private Library の `source_code`、および検証詳細の
  `adapter_source` / `case_results` は API の認可（所有者・開始者・admin）に従って表示されます。
  hidden ケースの入力・期待出力・stdout/stderr は返されません。
- **テキスト表示のみ**。Library / Version のソース、adapter、問題文は React のテキストノードと
  `<pre>{...}</pre>` で描画し、`innerHTML` / `dangerouslySetInnerHTML` は一切使いません。
  JSON レスポンスも `JSON.stringify` を `<pre>` に表示するだけです。

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
  `probeCookieSession` が dev ヘッダーを送らないこと、クライアントが組み立てる各エンドポイントの
  パス・メソッド・ボディ、slug の URL エンコード。
- `lib/verification.test.mjs` — 終端状態判定、ケース結果フォーマット、`case_results` の秘匿、
  ポーリング（即時終了 / pending→running→ac / 最大 30 回 / キャンセル）。

## 手動 E2E の目安

`DEV_HEADER_AUTH=true` のローカルで、seed（author `...0001` / admin `...0002`、Problem 1）を使います。
Problem 1 は先にテストケースを更新して `current_test_cases_key` を作っておきます（ルート README 参照）。
dev 認証をチェック → Namespace 作成 → private Library 作成 → 別ユーザーで 404 → public 化 →
Version 作成 → A+B adapter で verify → `ac` までポーリング、を確認します。

## 制限（設計どおりの MVP 範囲）

- Library Version は Python ソース 1 本（複数ファイル・複数言語は対象外）。
- fork / draft / publish / yank / ライセンス / API Token / Change Request / 検索は実装しません。
- 検証提出は通常の `submissions` として扱われ、提出一覧にも現れます。
- ブラウザ自動テスト（E2E）は含みません。UI の操作確認は上記の手動手順で行います。
- 画面は 1 ページ構成で、ルーティングや SSR データ取得は使っていません（クライアントコンポーネント）。
