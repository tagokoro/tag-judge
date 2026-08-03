"use client";

import Link from "next/link";
import { Edit, RefreshCcw, Trash2 } from "lucide-react";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { ErrorMessage } from "@/components/error-message";
import { StatusBadge, VisibilityBadge } from "@/components/status-badge";
import { SubmissionPanel } from "@/components/submission-panel";
import { TestCaseManager } from "@/components/test-case-manager";
import { api, ApiError } from "@/lib/api";
import type { Problem, Submission, TestCase } from "@/lib/types";

export default function ProblemDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const problemId = Number(params.id);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [testCases, setTestCases] = useState<TestCase[]>([]);
  const [submissions, setSubmissions] = useState<Submission[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [managerError, setManagerError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setManagerError(null);
    try {
      const nextProblem = await api.getProblem(problemId);
      setProblem(nextProblem);
      const [nextTestCases, nextSubmissions] = await Promise.all([
        api.listTestCases(problemId).catch((err: unknown) => {
          setManagerError(err instanceof ApiError ? err.message : "テストケースを取得できませんでした。");
          return [] as TestCase[];
        }),
        api.listSubmissions(problemId).catch(() => [] as Submission[])
      ]);
      setTestCases(nextTestCases);
      setSubmissions(nextSubmissions);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "問題を取得できませんでした。");
    } finally {
      setLoading(false);
    }
  }, [problemId]);

  useEffect(() => {
    if (Number.isFinite(problemId)) {
      void load();
    }
  }, [load, problemId]);

  async function removeProblem() {
    setError(null);
    try {
      await api.deleteProblem(problemId);
      router.push("/problems");
      router.refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "問題を削除できませんでした。");
    }
  }

  if (loading) {
    return <div className="panel p-4 text-sm text-ink/60">Loading...</div>;
  }

  if (!problem) {
    return <ErrorMessage message={error ?? "問題が見つかりません。"} />;
  }

  return (
    <div className="grid gap-6">
      <ErrorMessage message={error} />
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <section className="grid gap-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div className="mb-2 flex flex-wrap items-center gap-2">
                <span className="font-mono text-xs text-ink/50">#{problem.id}</span>
                <VisibilityBadge value={problem.visibility} />
              </div>
              <h1 className="text-2xl font-bold tracking-normal">{problem.title}</h1>
            </div>
            <div className="flex gap-2">
              <button className="button-secondary" type="button" onClick={load}>
                <RefreshCcw size={17} />
                Refresh
              </button>
              <Link className="button-secondary" href={`/problems/${problem.id}/edit`}>
                <Edit size={17} />
                Edit
              </Link>
              <button className="button-danger" type="button" onClick={removeProblem} title="Delete">
                <Trash2 size={17} />
              </button>
            </div>
          </div>
          <article className="panel p-4">
            <pre className="whitespace-pre-wrap text-sm leading-7 text-ink">{problem.statement}</pre>
          </article>
        </section>
        <aside className="grid content-start gap-3">
          <div className="panel p-4">
            <div className="label">Limits</div>
            <div className="mt-2 text-sm">
              <div>{problem.time_limit_ms} ms</div>
              <div>{problem.memory_limit_kb} KB</div>
            </div>
          </div>
          <div className="panel p-4">
            <div className="label">Owner</div>
            <div className="mt-2 break-all font-mono text-xs text-ink/65">{problem.created_by}</div>
          </div>
          <div className="panel p-4">
            <div className="label">Latest submission</div>
            <div className="mt-2">
              {submissions[0] ? <StatusBadge value={submissions[0].status} /> : <span className="text-sm text-ink/55">none</span>}
            </div>
          </div>
        </aside>
      </div>

      {managerError ? <ErrorMessage message={managerError} /> : <TestCaseManager problemId={problem.id} initialItems={testCases} onChange={load} />}
      <SubmissionPanel problemId={problem.id} initialSubmissions={submissions} />
    </div>
  );
}
