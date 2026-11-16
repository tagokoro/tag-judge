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
        <p className="page-sub">public な提出の最近のものです。提出日時から、ソースコードと結果を見られます。</p>
      </div>
      {submissions.length === 0 ? (
        <p className="empty">まだ提出がありません。</p>
      ) : (
        <div className="panel">
          <SubmissionTable showSubmitter submissions={submissions} />
        </div>
      )}
    </div>
  );
}
