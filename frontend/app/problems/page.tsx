"use client";

import Link from "next/link";
import { FilePlus, RefreshCcw } from "lucide-react";
import { useEffect, useState } from "react";
import { EmptyState } from "@/components/empty-state";
import { ErrorMessage } from "@/components/error-message";
import { VisibilityBadge } from "@/components/status-badge";
import { api, ApiError } from "@/lib/api";
import type { Problem } from "@/lib/types";

export default function ProblemsPage() {
  const [problems, setProblems] = useState<Problem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      setProblems(await api.listProblems());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "問題一覧を取得できませんでした。");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  return (
    <div className="grid gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-normal">Problems</h1>
          <p className="mt-1 text-sm text-ink/60">公開問題と作成済み問題を確認します。</p>
        </div>
        <div className="flex gap-2">
          <button className="button-secondary" type="button" onClick={load} disabled={loading}>
            <RefreshCcw size={17} />
            Refresh
          </button>
          <Link className="button-primary" href="/problems/new">
            <FilePlus size={17} />
            Create
          </Link>
        </div>
      </div>
      <ErrorMessage message={error} />
      {loading ? (
        <div className="panel p-4 text-sm text-ink/60">Loading...</div>
      ) : problems.length === 0 ? (
        <EmptyState title="問題はまだありません。" />
      ) : (
        <div className="overflow-hidden rounded-md border border-line bg-white shadow-soft">
          <table className="w-full min-w-[720px] border-collapse text-left text-sm">
            <thead className="bg-panel text-xs uppercase text-ink/55">
              <tr>
                <th className="px-4 py-3">ID</th>
                <th className="px-4 py-3">Title</th>
                <th className="px-4 py-3">Limits</th>
                <th className="px-4 py-3">Visibility</th>
                <th className="px-4 py-3">Updated</th>
              </tr>
            </thead>
            <tbody>
              {problems.map((problem) => (
                <tr key={problem.id} className="border-t border-line hover:bg-panel/70">
                  <td className="px-4 py-3 font-mono text-xs text-ink/55">{problem.id}</td>
                  <td className="px-4 py-3">
                    <Link className="font-semibold text-accent hover:underline" href={`/problems/${problem.id}`}>
                      {problem.title}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-ink/70">
                    {problem.time_limit_ms} ms / {problem.memory_limit_kb} KB
                  </td>
                  <td className="px-4 py-3">
                    <VisibilityBadge value={problem.visibility} />
                  </td>
                  <td className="px-4 py-3 text-ink/60">{new Date(problem.updated_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
