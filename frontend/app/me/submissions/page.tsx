"use client";

import { SubmissionTable } from "@/components/lists";
import { useAuth } from "@/components/AuthProvider";
import { useResource } from "@/components/useResource";
import { LoadState } from "@/components/ui";
import { LoginRequired } from "@/components/LoginRequired";

export default function MySubmissionsPage() {
  const { actor } = useAuth();
  const submissions = useResource(actor ? "me/submissions" : null, (api) => api.listMySubmissions());

  if (!actor) {
    return <LoginRequired title="自分の提出" />;
  }
  return (
    <div className="page">
      <h1>自分の提出</h1>
      {submissions.data ? (
        submissions.data.length === 0 ? (
          <p className="empty">まだ提出していません。</p>
        ) : (
          <div className="panel">
            <SubmissionTable
              submissions={submissions.data.map((submission) => ({
                ...submission,
                problem: `#${submission.problem_id}`,
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
