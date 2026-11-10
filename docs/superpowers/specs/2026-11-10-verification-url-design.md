# Library Version の提出URL設計

## 目的

利用者は普段の方法でコードを提出し、Library Version に提出URLを貼るだけで関連する提出を公開できる。Web UI で adapter のコードを書かせない。Library と Problem は独立したままにする。

## 決定

- `submission_url` は絶対 HTTP(S) URL として保存する。内部・外部の提出URLを区別しない。前後空白を除去し、ホストがないURL、制御文字を含むURL、2048バイトを超えるURLは拒否する。
- URLは取得・解析・判定しない。提出者・判定結果・使用したLibrary Versionの真正性を保証しない。画面では「関連する提出」と表現し、AC/Verify済みバッジは出さない。
- 認証済み利用者は公開LibraryのVersionにURLを追加できる。private Libraryには所有者だけが追加・閲覧できる。公開Libraryのリンク一覧・詳細は匿名でも閲覧できる。追加した利用者のIDと作成時刻を保存する。
- 同じVersionに複数のURLを登録できる。Problemの選択、adapterの入力、Library用のSubmission/Outbox作成、判定結果のポーリングは行わない。既存のSubmission APIとJudgeは変更しない。

## データとAPI

- `library_verifications` は `id`, `library_version_id`, `created_by`, `submission_url`, `created_at` を持つ。`submission_id`, `adapter_source`, `adapter_sha256` は削除する。検証済みの意味を避けるため、新しいコード・API・UIでは「提出リンク」と呼ぶ。既存PRのレビュー差分を小さくするためテーブル名は維持する。
- `POST /api/v1/namespaces/{namespace_slug}/libraries/{library_slug}/versions/{version_number}/verifications` は `{ "submission_url": "https://..." }` を受けてリンクを作成する。レスポンスはURL、Version識別子、登録者、作成時刻を返し、`submission_id`, `problem_id`, `status`, `score`, `adapter_source`, `case_results` は返さない。
- 同じVersionのリンク一覧を `GET` 同一パスで提供する。単一リンク取得は `GET /api/v1/library-verifications/{id}` を維持する。認可はリンクの属するLibraryの現在の公開範囲で判定する。
- URLの重複、削除、リンク先の失効確認は今回扱わない。

## PR境界

1. PR #44: migration、sqlc、domain/repository/PostgreSQL実装、DB統合テスト、設計書を変更する。
2. PR #45: Usecase、handler/router、OpenAPI、API構成、テストから判定投入依存を削除し、URL登録・取得へ変更する。
3. PR #46: Next.js基盤は維持する。積み上げ先だけ更新する。
4. PR #47: adapter入力とポーリングをURL入力・リンク一覧/詳細表示へ変更する。

## 受け入れ確認

- URLだけでVersionにリンクを登録でき、後から一覧・詳細で読める。
- private Libraryのリンクは非所有者から読めず、公開切り替えに追従する。
- 登録時にSubmission、Outbox、S3オブジェクトが新規作成されない。
- 不正URLは400、未認証での登録は401となる。
- 既存のJudge/Submissionのテストが引き続き通る。
