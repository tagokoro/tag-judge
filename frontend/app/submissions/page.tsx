"use client";

import { SubmissionTable } from "@/components/lists";
import { useResource } from "@/components/useResource";
import { LoadState } from "@/components/ui";

export default function SubmissionsPage() {
  const submissions = useResource("submissions/recent", (api) => api.listRecentSubmissions());

  return (
    <div className="page">
      <div className="page-head">
        <h1>Submissions</h1>
        <p className="page-sub">public な問題への最近の提出です。ソースコードは表示しません。</p>
      </div>
      {submissions.data ? (
        submissions.data.length === 0 ? (
          <p className="empty">まだ提出がありません。</p>
        ) : (
          <div className="panel">
            <SubmissionTable
              showSubmitter
              submissions={submissions.data.map((submission) => ({
                ...submission,
                problem: submission.problem_title,
                submitter: submission.submitter_namespace_slug,
              }))}
            />
          </div>
        )
      ) : (
        <LoadState loading={submissions.loading} error={submissions.error} notFound="" />
      )}
    </div>
  );
}
