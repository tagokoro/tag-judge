"use client";

import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { ErrorMessage } from "@/components/error-message";
import { ProblemForm } from "@/components/problem-form";
import { api, ApiError } from "@/lib/api";
import type { Problem } from "@/lib/types";

export default function EditProblemPage() {
  const params = useParams<{ id: string }>();
  const problemId = Number(params.id);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!Number.isFinite(problemId)) {
      setError("問題IDが不正です。");
      return;
    }
    api
      .getProblem(problemId)
      .then(setProblem)
      .catch((err: unknown) => setError(err instanceof ApiError ? err.message : "問題を取得できませんでした。"));
  }, [problemId]);

  return (
    <div className="grid gap-5">
      <div>
        <h1 className="text-2xl font-bold tracking-normal">Edit Problem</h1>
        <p className="mt-1 text-sm text-ink/60">問題本文と制約を更新します。</p>
      </div>
      <ErrorMessage message={error} />
      {problem ? (
        <div className="panel p-4">
          <ProblemForm problem={problem} />
        </div>
      ) : error ? null : (
        <div className="panel p-4 text-sm text-ink/60">Loading...</div>
      )}
    </div>
  );
}
