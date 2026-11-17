"use client";

import Link from "next/link";
import { formatDateTime, statusLabel, statusTone } from "@/lib/format";
import { isJudging } from "@/lib/judging";
import { problemPath, userPath } from "@/lib/routes";
import type { SubmissionDetail } from "@/lib/types";
import { CodeView } from "./CodeEditor";
import { usePolledSubmission, type PollStop } from "./usePolledSubmission";
import { Button, Card, LoadState, VisibilityBadge } from "./ui";

function StatusBadge({ status }: { status: string }) {
  return <span className={`status status-${statusTone(status)}`}>{statusLabel(status)}</span>;
}

/**
 * The submission page body, kept up to date while the submission is judged.
 * `initial` is what the server fetched with the visitor's cookies; without it
 * (a private submission seen through dev header auth) it loads here.
 */
export function SubmissionView({ submissionId, initial }: { submissionId: number; initial?: SubmissionDetail }) {
  const submission = usePolledSubmission(submissionId, initial);

  const data = submission.data;
  if (!data) {
    // Only reached without server data, i.e. through dev header auth (the
    // server already answered 404 for everyone else). Someone else's private
    // submission reads exactly like a missing one.
    return (
      <div className="page">
        <LoadState loading={submission.loading} error={submission.error} notFound="この提出は見つかりません。" />
      </div>
    );
  }

  return (
    <div className="page page-card">
      <div className="page-head">
        <div className="page-title">
          <h1>提出 #{data.id}</h1>
          <StatusBadge status={data.status} />
          {data.is_public ? null : <VisibilityBadge visibility="private" />}
        </div>
      </div>

      <Card title="結果">
        <dl className="kv">
          <dt>問題</dt>
          <dd>
            <Link href={problemPath(data.problem_id)}>{data.problem_title}</Link>
          </dd>
          <dt>提出者</dt>
          <dd>{data.submitter ? <Link href={userPath(data.submitter)}>@{data.submitter}</Link> : <span className="muted">—</span>}</dd>
          <dt>言語</dt>
          <dd>{data.language}</dd>
          <dt>提出日時</dt>
          <dd className="num">{formatDateTime(data.created_at)}</dd>
          <dt>得点</dt>
          <dd className="num">{data.score}</dd>
        </dl>
        {isJudging(data.status) ? <JudgingNote stopped={submission.stopped} onReload={submission.reload} /> : null}
      </Card>

      {data.case_results.length > 0 ? (
        <Card title="テストケース">
          <table className="table">
            <thead>
              <tr>
                <th scope="col" className="col-shrink">#</th>
                <th scope="col">結果</th>
                <th scope="col" className="col-shrink">実行時間</th>
              </tr>
            </thead>
            <tbody>
              {data.case_results.map((result, index) => (
                <tr key={result.test_case_id}>
                  <td className="num col-shrink" data-label="#">{index + 1}</td>
                  <td data-label="結果">
                    <StatusBadge status={result.status} />
                  </td>
                  <td className="num col-shrink" data-label="実行時間">
                    {result.execution_time_ms === null ? "—" : `${result.execution_time_ms} ms`}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      ) : null}

      <Card title="コード">
        {/* Source is untrusted text: CodeMirror shows it as text, never as HTML. */}
        <CodeView value={data.source_code} label="提出したコード" />
      </Card>
    </div>
  );
}

function JudgingNote({ stopped, onReload }: { stopped: PollStop | null; onReload: () => void }) {
  if (stopped === null) {
    return (
      <p className="note judging-note" role="status">
        判定中です。結果が出ると自動で表示します。
      </p>
    );
  }
  return (
    <div className="row judging-note">
      <p className={stopped === "error" ? "error" : "note"} role="status">
        {stopped === "error"
          ? "結果を取得できませんでした。"
          : "判定に時間がかかっています。しばらくしてから再読み込みしてください。"}
      </p>
      <Button onClick={onReload}>再読み込み</Button>
    </div>
  );
}
