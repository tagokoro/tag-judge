"use client";

import { ProblemList } from "@/components/lists";
import { useResource } from "@/components/useResource";
import { LoadState } from "@/components/ui";

export default function ProblemsPage() {
  const problems = useResource("problems", (api) => api.listProblems());

  return (
    <div className="page">
      <h1>問題一覧</h1>
      {problems.data ? (
        problems.data.length === 0 ? (
          <p className="empty">公開されている問題はまだありません。</p>
        ) : (
          <div className="panel">
            <ProblemList problems={problems.data} />
          </div>
        )
      ) : (
        <LoadState loading={problems.loading} error={problems.error} notFound="" />
      )}
    </div>
  );
}
