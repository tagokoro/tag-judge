# 提出リンクのマージ前整備

## 意図

未マージのライブラリ共有 PR を、提出 URL が「検証結果」ではなく自己申告の関連リンクであるという実態に合わせる。DB・API・画面の名称を揃え、非公開 Library のデータを DB 取得時点で保護する。

## 方針と範囲

- 新規テーブルをマージ前に `library_submission_links` と命名する。旧 `library_verifications` は作らず、データ移行用の追加 migration も作らない。API は `/submission-links`、ドメインと画面も Submission Link の名称に揃える。
- Library Version の作成では repository がソースから SHA-256 とバイト数を算出する。存在しない Library は `LIBRARY_NOT_FOUND` として返す。
- Version 本文・一覧、提出リンク本文・一覧の SQL は、同じ文で現在の Library 公開範囲と所有者を条件に含める。匿名・非所有者に非公開データを返さない。非公開／不存在は HTTP 404 とする。
- URL 検証は既存の `domain.ValidateSubmissionURL` をバックエンドの一元的な形式チェックとして維持する。同等の Go utility は存在しない。フロントのチェックは即時フィードバックのみで、サーバーが最終判定する。
- PR #44 に migration、sqlc クエリ・生成物、DB repository・統合テストを置く。PR #45 に API/usecase/OpenAPI、PR #46 に Next.js 基盤、PR #47 に UI を置く。

## 非対象

URL 先の取得、提出者や判定結果の証明、既存 Judge/Submission の変更、既存データの移行はしない。

## 検討した方式

- 旧名を残す: 差分は小さいが意味のずれが永続化するため採用しない。
- マージ後の追加 migration で改名: 今なら新規テーブル名を直接変更できるため採用しない。
- 新規テーブルと API 名をまとめて改名: 今回採用。積み上げ PR を DB から順に更新する。
