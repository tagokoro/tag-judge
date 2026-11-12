import type { Metadata } from "next";
import { SubmissionTable } from "@/components/lists";
import { serverClient } from "@/lib/serverApi";

export const metadata: Metadata = { title: "Submissions" };

export default async function SubmissionsPage() {
  const submissions = await (await serverClient()).listRecentSubmissions();

  return (
    <div className="page">
      <div className="page-head">
        <h1>Submissions</h1>
        <p className="page-sub">public な問題への最近の提出です。ソースコードは表示しません。</p>
      </div>
      {submissions.length === 0 ? (
        <p className="empty">まだ提出がありません。</p>
      ) : (
        <div className="panel">
          <SubmissionTable
            showSubmitter
            submissions={submissions.map((submission) => ({
              ...submission,
              problem: submission.problem_title,
              submitter: submission.submitter_namespace_slug,
            }))}
          />
        </div>
      )}
    </div>
  );
}
