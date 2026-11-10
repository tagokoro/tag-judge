# Tag Judge フロントエンド

Next.js（App Router）+ TypeScript のフロントエンドプロジェクトです。この
PR3 では環境・ツールと配信の統合だけを行い、画面はプレースホルダーのままです
（ライブラリ共有の UI は後続の PR で実装）。

## 必要なもの

- **Node.js 20.9.0 以上**（Next.js 16 の公式最小要件。`package.json` の
  `engines` にも明記）
- **npm 10 以上**（`package-lock.json` は lockfileVersion 3）

`package.json` / `package-lock.json` は依存を厳密に固定（exact version）して
いるため、再現可能なインストールには `npm ci` を使ってください。

## セットアップと起動

Go API（`localhost:8080`）と Next.js（`localhost:3000`）は**別プロセス**で
起動します。ルートの `make run-api` で Go API を、別ターミナルで以下を実行します。

```sh
cd frontend
npm ci
npm run dev
```

`npm run dev` は `http://localhost:3000/` で起動します。ブラウザは
`http://localhost:3000/api/*` だけを呼び、`next.config.ts` の rewrite が
サーバー側で Go API（`API_SERVER_ORIGIN`、開発既定 `http://localhost:8080`）
へ転送します。Google OAuth のコールバックも同じ rewrite を通るため、
`.env.example`（ルート）の `GOOGLE_OAUTH_REDIRECT_URL` は
`http://localhost:3000/api/v1/auth/google/callback` を指定します。

## スクリプト

| コマンド | 内容 |
| --- | --- |
| `npm run dev` | 開発サーバー（port 3000） |
| `npm run build` | 本番ビルド |
| `npm run start` | ビルド済みアプリの起動 |
| `npm run lint` | ESLint（`eslint-config-next`） |
| `npm run typecheck` | `tsc --noEmit` |

ルートの Makefile から `make frontend-install` / `make run-frontend` /
`make frontend-lint` / `make frontend-typecheck` / `make frontend-build`
でも実行できます。

## 環境変数

`API_SERVER_ORIGIN` は **サーバー側のみ**で利用します（`NEXT_PUBLIC_` を
付けない）。値は Next.js の rewrite 先として解決され、ブラウザには公開され
ません。ローカルのテンプレートは `.env.example` を参照してください。
