# MCP Server Design

Claude（Claude Code / Desktop / claude.ai）からTag Judgeの問題閲覧・作成、コード提出・結果取得を行うためのMCPサーバーの設計です。

**実装状況:** `/mcp`、閲覧・作成・提出のtool、OAuth 2.1の認可サーバーは実装済みです。`run_code` と、接続済みアプリの一覧・解除の画面は未実装です。

## 方針

- MCPサーバーは別プロセスにせず、Go APIに `/mcp` ルート（Streamable HTTP）として同居させる。
- toolは既存のusecaseを直接呼ぶ。HTTP APIを経由しない。
- 認証はOAuth 2.1で行う。Go APIが認可サーバーを兼ね、ログインには既存のGoogleログインを使う。claude.ai・Claude Desktop・Claude Codeのどれも、URLを登録するだけで接続できる。
- ブラウザから見えるURLはフロントのオリジンに揃える。`/mcp`、`/oauth/*`、`/.well-known/oauth-*` はNext.jsのrewriteでGo APIへ転送する。こうすると同意画面が、セッションCookieを持つオリジンで開く。

## 全体構成

```mermaid
flowchart TD
    Claude[Claude<br/>Code / Desktop / claude.ai] -->|HTTPS /mcp<br/>Authorization: Bearer| Front
    Claude -->|/.well-known/* /oauth/*| Front
    Browser[ブラウザ] -->|/api/v1/* /oauth/authorize<br/>Session Cookie| Front
    Front[Next.js<br/>rewrite] --> API

    subgraph API[Go API プロセス]
        AuthMW[認証ミドルウェア<br/>Cookie → Actor<br/>/mcp だけ Bearer → Actor]
        REST[既存 REST ハンドラ<br/>/api/v1/*]
        MCP[MCP ハンドラ<br/>/mcp]
        OAuth[認可サーバー<br/>/oauth/* /.well-known/*]
        UC[usecase 層]
        AuthMW --> REST
        AuthMW --> MCP
        AuthMW --> OAuth
        REST --> UC
        MCP --> UC
    end

    UC --> DB[(PostgreSQL<br/>oauth_* を追加)]
    OAuth --> DB
    UC --> S3[S3]
    DB --> Publisher[Outbox Publisher]
    Publisher --> SQS[SQS]
    SQS --> Worker[Rust judge-worker]
    Worker -->|判定結果を保存| DB
    OAuth -->|未ログインなら| Google[Google ログイン]
```

## 使い方

ユーザーは、Claudeに `https://<フロントのドメイン>/mcp` を登録するだけです。

- **claude.ai / Claude Desktop**：設定 → コネクタ → カスタムコネクタを追加 → URLを入力 → 接続
- **Claude Code**：`claude mcp add --transport http tag-judge https://<ドメイン>/mcp` を実行し、`/mcp` から認証する

接続するとブラウザで同意画面が開きます（未ログインならGoogleログインを挟みます）。「許可」を押すと接続が完了します。

### ローカルで試す

```sh
set -a; source .env; set +a; make run-api   # 別ターミナルで make run-frontend も起動する
claude mcp add --transport http tag-judge http://localhost:3000/mcp
# Claude Code で /mcp → tag-judge → Authenticate
```

claude.ai はAnthropicのサーバーから接続するため、`localhost` には届きません。デプロイ先か、トンネル（cloudflared など）で公開したURLを使ってください。

### 設定

| 環境変数 | 説明 |
| --- | --- |
| `PUBLIC_BASE_URL` | ユーザーとClaudeがアクセスするオリジン。OAuthのissuerで、その `/mcp` がMCPのURLになる。省略すると `AUTH_SUCCESS_REDIRECT_URL` のオリジンを使う |
| `MCP_SUBMIT_WAIT_SECONDS` | `submit_code` の待機上限（既定8秒）。`WRITE_TIMEOUT`（既定10秒）以上にすると起動時にエラーになる |

### エンドポイント

| エンドポイント | 説明 |
| --- | --- |
| `POST /mcp` | MCP（Stateless・JSON応答のStreamable HTTP）。トークンがないと `401` と `WWW-Authenticate: Bearer resource_metadata="..."` を返す |
| `GET /.well-known/oauth-protected-resource`（`/mcp` 付きも可） | 保護リソースのメタデータ（RFC 9728） |
| `GET /.well-known/oauth-authorization-server` | 認可サーバーのメタデータ（RFC 8414） |
| `POST /oauth/register` | 動的クライアント登録（RFC 7591）。public client（`none`）と、`client_secret_post` / `client_secret_basic` に対応 |
| `GET /oauth/authorize` | 同意画面。PKCE（S256）は必須。未ログインならGoogleログインへ送り、ログイン後に戻す |
| `POST /oauth/authorize` | 同意フォームの送信先（CSRFトークンつき） |
| `POST /oauth/token` | `authorization_code` と `refresh_token` のgrant |

- 認可コードは5分、アクセストークンは1時間、リフレッシュトークンは30日で失効する。
- リフレッシュするたびにトークンの組を作り直す。古いアクセストークンとリフレッシュトークンは、その時点で使えなくなる。
- 認可コードとトークンは、DBにはSHA-256ハッシュだけを保存する。アクセストークンは `tja_`、リフレッシュトークンは `tjr_` で始まる。
- `redirect_uri` は登録済みのものと完全一致で照合する。登録できるのは、https のURL、ループバック（`localhost` など）上の http のURL、アプリ独自のスキーム（Cursor の `cursor://...` など。RFC 8252）。`javascript:` や `file:` などブラウザが自分で扱うスキームは拒否する。
- 同意できるのはブラウザのセッションだけ。同意画面は `X-Frame-Options: DENY` で、iframeへの埋め込みを禁止している。
- `/mcp` はブラウザのセッションCookieを受け付けない。ブラウザで開いたページから、ユーザーのCookieでtoolを動かされないようにするため。
- `/mcp` をブラウザで開いた人（`Accept: text/html`）は、Next.jsのredirectで接続手順のページ（`/connect`）へ送る。MCPクライアントはHTMLを求めないので影響しない。
- 新規ユーザーがClaudeから接続した場合は、ハンドル登録（`/signup`）を挟んでから同意画面に戻る。

## tool一覧

`/mcp` を使うには接続（OAuth）が必要です。toolはすべて、接続を許可したユーザーとして動きます。

| tool | usecase | 必要な権限 |
| --- | --- | --- |
| `list_problems` | ListPublicProblems（`mine=true` で ListMyProblems） | なし |
| `get_problem` | GetProblem | 非公開の問題は管理者だけ |
| `create_problem` | CreateProblem（visibility の既定は private） | author / admin、ハンドル登録済み |
| `update_problem` | UpdateProblem（公開もこれで行う） | 問題の管理者 |
| `list_test_cases` | TestCases.List | 問題の管理者 |
| `add_test_case` | TestCases.Create | 問題の管理者 |
| `submit_code` | Submissions.Create（判定完了まで上限つきで待つ） | なし（提出の流量制限あり） |
| `get_submission` | Submissions.Get | 非公開の提出は本人だけ |
| `run_code` | CodeRuns.Create（Workerが対応するまで公開しない） | 未実装 |

## 認証ミドルウェアの判定フロー

```mermaid
flowchart TD
    Start([リクエスト受信]) --> HasCookie{Session Cookie<br/>がある?}
    HasCookie -->|Yes| CheckSession{sessions に<br/>一致する?}
    CheckSession -->|Yes| SetActor[Actor を context に設定]
    CheckSession -->|No| DevHeader
    HasCookie -->|No| DevHeader{DEV_HEADER_AUTH=true<br/>かつ X-User-ID あり?}
    DevHeader -->|Yes| SetActor
    DevHeader -->|No| Anonymous[未認証のまま続行]
    SetActor --> IsMCP{/mcp ?}
    Anonymous --> IsMCP
    IsMCP -->|No| Next([ハンドラへ])
    IsMCP -->|Yes| HasBearer{Authorization:<br/>Bearer がある?}
    HasBearer -->|Yes| CheckToken{oauth_tokens に一致し、<br/>失効・期限切れでない?}
    CheckToken -->|Yes| UpdateUsed[last_used_at を更新] --> TokenActor[トークンの Actor で上書き] --> MCPNext([MCP ハンドラへ])
    CheckToken -->|No| Reject[401 + WWW-Authenticate<br/>error=invalid_token]
    CheckToken -->|DB エラー| Fail[500]
    HasBearer -->|No| Challenge[401 + WWW-Authenticate<br/>resource_metadata]
```

- Bearerを受け付けるのは `/mcp` だけ。トークンで許すのはMCPのtoolでできる操作に限るため、REST API（`/api/v1/*`）ではBearerを見ない。
- 不正なBearerは401で拒否する。Claudeがトークンの更新か再認証を始められるようにするため。
- トークンの照合でDBが失敗したときは500を返す。401を返すと、一時的な障害で全クライアントが再認証を始めてしまう。
- `/mcp` はセッションCookieを受け付けない。ブラウザで開いたページが、利用者のCookieでtoolを動かせないようにするため。

## ① 接続（OAuth）

```mermaid
sequenceDiagram
    participant C as Claude
    participant B as ブラウザ
    participant API as Go API<br/>（認可サーバー兼任）
    participant G as Google

    C->>API: POST /mcp（トークンなし）
    API-->>C: 401 + WWW-Authenticate（resource_metadata）
    C->>API: GET /.well-known/oauth-protected-resource
    C->>API: GET /.well-known/oauth-authorization-server
    C->>API: POST /oauth/register（動的クライアント登録）
    API-->>C: client_id
    C->>B: /oauth/authorize?client_id&redirect_uri&code_challenge&state を開く
    B->>API: GET /oauth/authorize
    alt セッションなし
        API-->>B: /api/v1/auth/google/login?next=/oauth/authorize?...
        B->>G: ログイン
        G-->>B: callback
        B->>API: GET /api/v1/auth/google/callback
        API-->>B: next へ戻す（新規ユーザーは /signup を挟む）
        B->>API: GET /oauth/authorize
    end
    API-->>B: 同意画面（CSRFトークン）
    B->>API: POST /oauth/authorize（許可）
    API-->>B: redirect_uri?code&state&iss
    B-->>C: 認可コード
    C->>API: POST /oauth/token（code + code_verifier）
    API->>API: PKCE を検証し、トークンを発行
    API-->>C: access_token（1時間）・refresh_token（30日）
    C->>API: POST /mcp（Bearer access_token）
    Note over C,API: 期限が切れたら refresh_token で更新する
```

## ② MCPの初期化

```mermaid
sequenceDiagram
    participant C as Claude
    participant MW as 認証ミドルウェア
    participant M as MCP ハンドラ

    C->>MW: POST /mcp initialize（Bearer）
    MW->>MW: Bearer → Actor
    MW->>M: initialize
    M-->>C: サーバー情報・capabilities
    C->>M: tools/list
    M-->>C: tool 一覧（名前・説明・入力スキーマ）
```

## ③ 提出から判定結果まで

```mermaid
sequenceDiagram
    participant C as Claude
    participant M as MCP ハンドラ<br/>submit_code
    participant UC as usecase
    participant S3 as S3
    participant DB as PostgreSQL
    participant Q as Outbox → SQS
    participant W as judge-worker

    C->>M: tools/call submit_code(problem_id, source_code)
    M->>UC: Submissions.Create(actor, ...)
    UC->>UC: admission control（流量制限）
    UC->>S3: 提出コード・テストケースsnapshotを保存
    UC->>DB: Submission + Outbox を同一トランザクションで保存
    UC-->>M: submission_id（status: pending）
    DB-->>Q: Outbox Publisher が発行
    Q->>W: ジョブ配送
    W->>W: 判定（judge-core）

    loop 判定完了まで（最大 30 秒、数百 ms 間隔）
        M->>UC: Submissions.Get(submission_id)
        UC->>DB: 状態を取得
        DB-->>M: status
    end

    W->>DB: 判定結果をトランザクションで保存

    alt 時間内に確定
        M-->>C: verdict・得点・ケース別結果
    else 時間切れ
        M-->>C: status: pending, submission_id
        C->>M: tools/call get_submission(submission_id)
        M-->>C: 最新の状態・結果
    end
```

## submit_code の内部フロー

```mermaid
flowchart TD
    Start([submit_code 呼び出し]) --> Auth{Actor がある?}
    Auth -->|No| ErrAuth[tool エラー: 認証が必要]
    Auth -->|Yes| Validate{入力が正しい?<br/>problem_id / source_code}
    Validate -->|No| ErrInput[tool エラー: 入力不正]
    Validate -->|Yes| Create[Submissions.Create]
    Create --> Admit{admission control<br/>を通過?}
    Admit -->|No| ErrRate[tool エラー: 制限超過<br/>再試行までの目安を返す]
    Admit -->|Yes| Wait[状態を確認]
    Wait --> Done{判定が確定?}
    Done -->|Yes| Result[verdict・ケース別結果を返す]
    Done -->|No| Timeout{待機上限を超えた?<br/>またはクライアント切断}
    Timeout -->|No| Sleep[数百 ms 待つ] --> Wait
    Timeout -->|Yes| Pending[status: pending と<br/>submission_id を返す]
```

- 待機の上限は、サーバーの `WRITE_TIMEOUT` とALBのidle timeoutより短くする。
- クライアントが切断したら（`ctx.Done()`）待機をやめる。提出自体はOutbox経由で判定まで進む。

## 今後の予定

1. 接続済みアプリの一覧と解除の画面（`oauth_tokens` をユーザーごとに失効させる）
2. MCP経由のアクセスログと、閲覧系toolのレート制限
3. 動的クライアント登録の流量制限と、使われていないクライアント・期限切れトークンの掃除
4. Workerが対応したら `run_code`
