# 問題提出フロー 仕様

問題の詳細ページ、問題への提出、提出結果の閲覧、任意の入力でのコード実行の仕様です。
受付制御（レート制限・Idempotency-Key・outbox）は [Admission Control](admission-control.md)、ジャッジの構成は [Rust Judge 設計](rust-judge-design.md)、HTTP API の詳細は [OpenAPI](../backend/docs/openapi.yaml) を正とします。

## 目的

- public な問題と、自分が作った private な問題に提出できるようにする。
- private な問題への提出は、提出した本人だけが見られるようにする。
- public な問題への提出は、ソースコードも含めてだれでも見られるようにする。
- 問題ページの「コード実行」タブと単独ページ `/code-runs` で、任意の入力でコードを実行できるようにする。

## 現状と問題点（main `aebad3b` 時点）

| 項目 | 現状 | 問題 |
| --- | --- | --- |
| `GET /problems/{id}` | public のみ（`GetPublicProblem`） | 作成者が自分の private 問題を見られない |
| `GET /problems/{id}/test-cases` | 問題の管理者のみ。非公開ケースも返す | 管理者以外がサンプルを読む手段がない |
| `POST /problems/{id}/submissions` | public のみ（`FindPublicByID`） | 作成者でも private 問題に提出できない |
| `GET /submissions/{id}` | 本人と admin のみ。それ以外は **403** | 他人の private リソースを 404 にする方針と合わない。ソースコードを返さない（`source_code_key` だけ） |
| ソース・stdin のサイズ | 上限なし | stdin は SQS メッセージにそのまま入るので、256 KiB を超えると送れない。body のサイズ上限もない |
| Rust worker | `JudgeJob`（`submissionId` 必須）としてのみパース | CodeRun は実行されない。CodeRun のメッセージを受けると `receive_job()?` でプロセスが終了し、メッセージが再配信されるたびに落ち続ける |
| stdout / stderr | 切り詰めない | 大量出力がそのまま `code_runs` に入る |

## 決めたこと

| 論点 | 決定 |
| --- | --- |
| private の意味 | 下書き（draft）。作成者だけが見られ、公開前の確認のために作成者だけが提出できる |
| visibility の変更 | private / group → public の一方向だけ。**public から private / group には戻せない**（admin も不可） |
| 問題の閲覧 | public はだれでも（未ログイン含む）。private / group は作成者と admin のみ。それ以外は 404 |
| 提出 | public はログイン済みならだれでも。private / group は作成者のみ（admin も他人の private 問題には提出できない）。それ以外は 404 |
| group | グループ所属による権限は今回扱わず、private と同じ扱いにする |
| public 問題への提出 | ソースコードを含めてだれでも（未ログイン含む）閲覧できる |
| private 問題への提出 | 提出者本人と admin のみ。他人には 404。`/submissions/recent` や他人の一覧には出さない |
| 提出の可視性 | **提出した時点で決めて保存する**（`submissions.is_public`）。問題を後から公開しても、下書きの間の提出は非公開のまま |
| 公開後に問題が壊れていたら | 非公開には戻さず、問題文・テストケースを直す。公開停止の状態は今回作らない |
| コード実行 | 問題には紐づけない（既存の `code-runs` API）。問題ページのタブと単独ページ `/code-runs` の 2 か所に置く |
| worker | 今回のスコープに含める。CodeRun を実行し、パースできないメッセージで落ちないようにする |

### visibility と提出の可視性

- `PATCH /problems/{id}` で、public の問題の visibility を private / group にしようとしたら 409 `PROBLEM_ALREADY_PUBLIC` を返す。public → public（変更なし）は通す。
- public にするときの既存の条件（テストケースが 1 件以上）はそのまま。
- visibility が狭まる方向に変わらないので、公開された提出のリンクや `/submissions/recent` が後から 404 になることはない。
- 提出の作成時に `is_public = (problem.visibility == 'public')` を入れ、以後は変えない。下書きの間の提出（作成者が確認のために出したもの）は、公開後も作成者本人と admin にしか見えない。
- 問題を読んでから提出を入れるまでの間に問題が公開されても、その提出は非公開になるだけで、公開すべきでない提出が公開されることはない。
- 提出のある問題は今も削除できない（`submissions.problem_id` に `ON DELETE` がない）ので、public の問題は実質ずっと残る。壊れた問題の扱い（公開停止の状態など）は、必要になったら別に決める。

### マイグレーション

```sql
-- 000012_add_submission_is_public.up.sql
ALTER TABLE submissions ADD COLUMN is_public boolean NOT NULL DEFAULT false;
UPDATE submissions s SET is_public = true
FROM problems p WHERE p.id = s.problem_id AND p.visibility = 'public';
CREATE INDEX submissions_public_created_at_idx ON submissions (created_at DESC, id DESC) WHERE is_public;
```

- 既存の提出は、問題の現在の visibility から埋める（未リリースなので、過去の visibility の履歴は追わない）。
- `DEFAULT false` は、値を入れ忘れたときに非公開になる側に倒すため。作成の SQL では必ず明示的に入れる。

## 権限の実装

判定は usecase の 1 か所にまとめる（`submission_usecase.go` のコメント「Problem access policy belongs here」の場所）。

```go
// usecase/problem_access.go
func canViewProblem(actor auth.Actor, p domain.Problem) bool   // public || 作成者 || admin
func canSubmitProblem(actor auth.Actor, p domain.Problem) bool // public || 作成者
func canViewSubmission(actor auth.Actor, s domain.Submission) bool
    // s.IsPublic || s の提出者 || admin（問題の現在の visibility は見ない）
```

- 見えない場合は存在しない場合と同じ 404 を返す（`PROBLEM_NOT_FOUND` / `SUBMISSION_NOT_FOUND`）。見えるが提出できない場合はない（見える private 問題は作成者か admin のもので、admin が他人の private 問題に提出したときだけ 404 にする）。
- `FindPublicByID` での絞り込みをやめ、`FindByID` で取ってから上の関数で判定する。SQL 側には権限を持たせない（一覧系は SQL で絞る。`ListPublic` は従来どおり `visibility = 'public'`、`ListRecentPublic` は問題との JOIN の条件をやめて `s.is_public` で絞る）。
- 未ログインの actor は、既存の Library API と同じくゼロ値の `auth.Actor` で表す。
- 管理系（`PATCH` / `DELETE /problems/{id}`、テストケースの CRUD）でも、見えない問題（他人の private / group）は 403 ではなく 404 にする。見える問題（public）を作成者・admin 以外が管理しようとしたときは従来どおり 403。

## API の変更

| エンドポイント | 認証 | 変更 |
| --- | --- | --- |
| `GET /problems/{id}` | 任意 | 上の閲覧権限で判定。レスポンスに `samples` を追加 |
| `PATCH /problems/{id}` | 必須 | public → private / group を 409 `PROBLEM_ALREADY_PUBLIC` で拒否 |
| `POST /problems/{id}/submissions` | 必須 | 上の提出権限で判定。サイズ上限 |
| `GET /submissions/{id}` | **任意に変更** | 上の閲覧権限で判定、403 → 404。レスポンスを変更（下記） |
| `GET /submissions` | 必須 | `?problem_id=` で絞り込めるようにする（問題ページの「自分の提出」用） |
| `POST /code-runs` | 必須 | サイズ上限 |
| `GET /code-runs/{id}` | 必須 | 他人のものは 403 → 404 |

### `GET /problems/{id}` の `samples`

```json
{
  "id": 1, "title": "...", "statement": "...", "time_limit_ms": 2000, "memory_limit_kb": 262144,
  "visibility": "public", "created_by": "seed-author", "created_at": "...", "updated_at": "...",
  "samples": [{ "input": "1 2\n", "output": "3\n" }]
}
```

- `is_sample = true` のテストケースを id 順に返す。サンプルがなければ空配列。
- `samples` を付けるのは 1 件の取得（`GET /problems/{id}`）だけ。一覧・作成・更新は従来の形（`problemResponse`）のまま。
- 既存の `GET /problems/{id}/test-cases` は管理用（管理者のみ、非公開ケースと `score` を含む）のまま変えない。
- サンプルでないテストケースの中身はどこにも出さない（従来どおり）。

### `GET /submissions/{id}` のレスポンス

```json
{
  "id": 10, "problem_id": 1, "problem_title": "A + B", "submitter": "alice",
  "language": "python", "source_code": "print(sum(map(int, input().split())))",
  "status": "ac", "score": 100, "is_public": true, "created_at": "...", "judged_at": "...",
  "case_results": [{ "test_case_id": 3, "status": "ac", "execution_time_ms": 12, "memory_kb": null }]
}
```

- `user_id` と `source_code_key`（内部のキー）は返さない。提出者はハンドル `submitter` で表す（ハンドルがなければ `null`）。[ユーザープロフィール](user-profile.md) の方針に合わせる。
- `source_code` は API がストレージ（S3 / ローカル）から読む。`SourceCodeStorage` に `GetSourceCode` を追加する。
- `case_results` は従来どおり状態と時間だけで、stdout は返さない（非公開ケースの入力が漏れるため）。
- 作成直後の `POST` のレスポンスと `GET /submissions`（自分の一覧）は今回は変えない。

### サイズ上限

| 対象 | 上限 | エラー |
| --- | --- | --- |
| `source_code`（提出・実行） | 64 KiB（UTF-8 バイト数） | 400 `SOURCE_CODE_TOO_LARGE` |
| `stdin`（実行） | 128 KiB | 400 `STDIN_TOO_LARGE` |
| リクエスト body | 約 1.2 MiB（`6 × (64 KiB + 128 KiB) + 64 KiB`） | 400 `INVALID_REQUEST_BODY` |
| stdout / stderr（実行結果） | それぞれ 64 KiB。worker が UTF-8 の境界で切り詰める | — |

stdin は SQS メッセージ（上限 256 KiB）に入るので、JSON エスケープで膨らむ分を見込んで 128 KiB にする。上限は定数（`domain.MaxExecutionSourceBytes` / `domain.MaxCodeRunStdinBytes`）にし、環境変数にはしない。

body の上限は既存の Library API（`decodeLibraryJSON`）と同じ考え方で、すべて `\uXXXX` でエスケープされても通る大きさにし、超えたら不正な JSON と同じ 400 にする。デコード後の大きさは usecase が上の上限で確かめる。

### レート制限・時間制限

- 提出とコード実行は既存の受付制御を共有する（1 ユーザー 60 秒に 10 件、実行中 2 件まで）。フロントは 429 の `Retry-After` を使って「n 秒後に再度お試しください」と出す。
- コード実行の時間制限は `CODE_RUN_TIME_LIMIT_MS`（既定 5000 ms）のまま。問題ページから実行しても問題の時間制限は使わない（問題に紐づけないため）。メモリは isolate の固定値（256 MiB）。

## Worker（CodeRun の実行）

1. **メッセージの判別**：`reference.kind`（`submission` / `code_run`）で `ExecutionJob::Submission(JudgeJob)` と `ExecutionJob::CodeRun(CodeRunJob)` に分ける。API はすでに両方の形式で `reference` を送っている。
2. **パースできないメッセージ**：プロセスを終了せず、ログに出して次のメッセージに進む。削除はしないので、再配信を繰り返したあと既存の DLQ（`submissions_dlq`）に移る。
3. **CodeRun の実行**：`sourceCodeKey` から S3 でソースを読み、`Runner::run`（`input = stdin`）で 1 回実行する。結果は次のように対応させる。

   | 実行結果 | `code_runs.status` |
   | --- | --- |
   | 時間切れ | `tle` |
   | 終了コード 0 以外 | `re` |
   | 正常終了 | `ac`（画面では「正常終了」と表示） |
   | runner の起動失敗など | `ie` |

4. **結果の書き込み**：`ResultSink` に CodeRun 用の `mark_code_run_running` / `complete_code_run` を足す。提出と同じく `pending` / `running` のときだけ更新し、終了済みなら重複メッセージとして何もせず削除する。stdout / stderr を 64 KiB で切り詰め、`execution_time_ms` を入れる（`memory_kb` は NULL のまま）。
5. S3 の読み込みや DB の一時的な失敗は、提出と同じくメッセージを残して再試行する。

## 画面

| URL | 内容 | 描画 |
| --- | --- | --- |
| `/problems/{id}` | 問題ページ。タブ：問題文 / 提出 / コード実行（`?tab=submit` / `?tab=run`） | 問題の取得は Server Component、タブの中身はクライアント |
| `/submissions/{id}` | 提出結果ページ | 初回は Server Component、判定中はクライアントでポーリング |
| `/code-runs` | コード実行の単独ページ（実行履歴つき） | クライアント（要ログイン） |

- 問題・提出が見つからない（または見えない）ときは `notFound()` で共通の 404 ページを出す。「作成者だけ」などの文言は出さない。
- 問題一覧・自分の問題・提出一覧の各行から、`/problems/{id}` と `/submissions/{id}` にリンクする。ヘッダーに「コード実行」（`/code-runs`）を足す。
- URL は `lib/routes.ts` に `problemPath(id)` / `submissionPath(id)` / `CODE_RUNS_PATH` として足す。

### 問題ページ `/problems/{id}`

- **問題文タブ**：タイトル、作成者（ハンドル）、時間・メモリ制限、private なら「private」バッジ、問題文（既存の `RichText`、Markdown + KaTeX）、サンプル（入力・出力とコピー用ボタン）。
- **提出タブ**：言語（今は Python のみ）、エディタ（既存の `CodeEditor`）、「提出」ボタン。
  - 提出は `Idempotency-Key`（クリックごとに `crypto.randomUUID()`、通信エラーで再試行するときは同じキー）を付けて送り、成功したら `/submissions/{id}` に移る。
  - エディタの下に、この問題への自分の提出（`GET /submissions?problem_id=`）を新しい順に出す。
  - 未ログインなら、エディタの代わりに既存の `LoginRequired` を出す。
  - 409 `PROBLEM_HAS_NO_TEST_CASES` は「この問題にはまだテストケースがないため提出できません」と出す。
- **コード実行タブ**：エディタ、標準入力、「実行」ボタン、結果（状態・実行時間・stdout・stderr）。
  - 「サンプル n を入力」ボタンで標準入力にサンプルを入れる。そのまま実行した場合は、stdout をサンプルの出力と比べて「一致 / 不一致」を出す（末尾の空白・改行は無視）。
  - 未ログインなら `LoginRequired`。
- 提出タブとコード実行タブは**同じソースコード**を編集する。ソースは問題ごとに `localStorage` に下書きとして保存する（読み書きは try/catch で囲み、失敗しても動くようにする）。
- タブは URL の `?tab=submit` だけで持ち、`useSearchParams` で読む。切り替えは `window.history.replaceState` で URL を置き換える（履歴は積まない。Next.js 16 では `replaceState` が `useSearchParams` に反映される）。actor が変わってページが作り直されても、タブは URL のとおりに保たれる。
- dev ヘッダー認証の actor はサーバーから見えないため、開発ビルドではサーバーで 404 だった問題・提出もブラウザ側で取り直す（既存の Library ページと同じ）。そのときブラウザ側では `notFound()` を呼ばず「見つかりません」の文言を出し、actor を切り替えたら取り直す。本番ビルドはサーバーがそのまま 404 を返す。

### 提出結果ページ `/submissions/{id}`

- 問題（リンク）、提出者、言語、状態、得点、提出日時、ソースコード（読み取り専用のエディタ）、テストケースごとの結果。
- 判定中（`pending` / `running`）の間はポーリングして、終わったら止める。

### `/code-runs`

- 問題ページのコード実行タブと同じ部品（エディタ・標準入力・結果）に、自分の実行履歴（`GET /code-runs`）を加えたページ。履歴の行を選ぶと、そのときのソースと入力をエディタに読み込む。
- 下書きは問題ページとは別のキーで `localStorage` に保存する。

### ポーリング

- 提出結果とコード実行の結果で共通の `lib/poll.ts` を使う。
- 間隔は 0.5 秒から始めて 1.5 倍ずつ伸ばし、上限 3 秒。終了状態になったら止める。
- タブが非表示（`document.hidden`）の間は止め、表示されたらすぐ 1 回取得する。ページを離れたら `AbortController` で止める。
- 2 分たっても終わらなければ止めて、「判定に時間がかかっています」と「再読み込み」ボタンを出す。
- 取得の失敗（5xx・通信エラー）は 3 回まで間隔を伸ばして続け、それを超えたら止めてエラーを出す。

## テスト

- **usecase のテスト**（`*_usecase_test.go`、テーブル駆動）
  - visibility（public / private / group）× actor（未ログイン / 作成者 / 他人 / admin）で、問題の閲覧・提出・提出の閲覧の可否と、拒否時に 404 になること。
  - public → private / group の変更が 409 になること（作成者・admin とも）。private → public は通ること。
  - 下書きの間の提出が、問題の公開後も他人から 404 のままであること。公開後の提出は公開されること。
  - サイズ上限の境界（ちょうど上限は通る、1 バイト超えは 400）。
- **PostgreSQL の integration test**（既存の `*_integration_test.go` と同じく専用スキーマ）
  - サンプルの取得（`is_sample` だけが返ること）、提出の詳細取得（問題タイトル・ハンドルの JOIN）、`problem_id` での絞り込み。
  - `is_public = false` の提出が `/submissions/recent` に出ないこと（既存テストを書き換える）。
  - マイグレーションの埋め方（public 問題への既存の提出だけが `is_public = true` になること）。
- **router のテスト**：`GET /submissions/{id}`・`GET /problems/{id}` が未ログインで呼べること。`GET /problems/{id}` だけが `samples` を返すこと。上限を超える body が 400 `INVALID_REQUEST_BODY` に、エスケープされた上限超えのソースが 400 `SOURCE_CODE_TOO_LARGE` になること。
- **worker のテスト**（Rust）
  - submission と code_run のメッセージのパース。パースできないメッセージでループが終わらないこと。
  - モック runner で `ac` / `re` / `tle` / `ie` になること。stdout の切り詰めが UTF-8 の境界で行われること。
  - 終了済みの CodeRun への重複メッセージで何も更新しないこと。
- **フロントのテスト**（`node:test`、`lib/*.test.mjs`）
  - ポーリングの間隔の計算・停止条件、出力の一致判定、エラー（429・409・404）から表示文言への変換、`routes.ts` の追加分。
- **手動確認**：ローカル環境（`docs/local-development.md`）で、public / private 問題への提出、別ユーザーでの 404、未ログインでの閲覧、コード実行の結果表示を確かめる。

## PR の分け方

feature ブランチを順に積み、それぞれマージコミットでマージする。

1. **backend**：`submissions.is_public` のマイグレーション、public から戻す変更の禁止、権限の判定、`GET /problems/{id}` の `samples`、`GET /submissions/{id}` の変更、`problem_id` での絞り込み、サイズ上限、OpenAPI。
2. **frontend（問題・提出）**：`/problems/{id}`（問題文・提出タブ）、`/submissions/{id}`、ポーリング、一覧からのリンク。
3. **worker**：メッセージの判別、CodeRun の実行と結果の書き込み、パースできないメッセージの扱い。
4. **frontend（コード実行）**：コード実行タブ、`/code-runs`、ヘッダーのリンク。

2 は 1 の API だけを使う。3 は 1・2 と独立していて、CodeRun のメッセージで worker が落ちる問題を直すので、先に入れられるなら先に入れる。

## 今回やらないこと

- グループ所属による閲覧・提出の権限。
- 問題ごとの全員の提出一覧（`/submissions/recent` の `problem_id` での絞り込み）。
- 問題の作成・編集画面。作るときは、public にする操作に「公開すると非公開に戻せません」と表示する。
- 公開停止（archived）の状態。壊れた public 問題を隠す必要が出たら決める。
- コード実行のメモリ使用量の計測、Python 以外の言語。
- 問題を削除したときの提出の扱い（`submissions.problem_id` に `ON DELETE` がないため、提出のある問題は今も削除できない）。
