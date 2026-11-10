"use client";

import { ProblemList } from "@/components/lists";
import { useAuth } from "@/components/AuthProvider";
import { useResource } from "@/components/useResource";
import { LoadState } from "@/components/ui";
import { LoginRequired } from "@/components/LoginRequired";

export default function MyProblemsPage() {
  const { actor } = useAuth();
  const problems = useResource(actor ? "me/problems" : null, (api) => api.listMyProblems());

  if (!actor) {
    return <LoginRequired title="自分の問題" />;
  }
  return (
    <div className="page">
      <div className="page-head">
        <h1>自分の問題</h1>
        <p className="page-sub">自分が作成した問題です。非公開の問題も含みます。</p>
      </div>
      {problems.data ? (
        problems.data.length === 0 ? (
          <p className="empty">まだ問題を作成していません。</p>
        ) : (
          <div className="panel">
            <ProblemList problems={problems.data} showVisibility />
          </div>
        )
      ) : (
        <LoadState loading={problems.loading} error={problems.error} notFound="" />
      )}
    </div>
  );
}
