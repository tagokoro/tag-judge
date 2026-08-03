"use client";

import { Play, RefreshCcw } from "lucide-react";
import { useEffect, useState } from "react";
import { EmptyState } from "@/components/empty-state";
import { ErrorMessage } from "@/components/error-message";
import { StatusBadge } from "@/components/status-badge";
import { api, ApiError } from "@/lib/api";
import type { Submission } from "@/lib/types";
import { validateSubmissionSource } from "@/lib/validation";

const starterCode = `a, b = map(int, input().split())
print(a + b)
`;

export function SubmissionPanel({ problemId, initialSubmissions }: { problemId: number; initialSubmissions: Submission[] }) {
  const [sourceCode, setSourceCode] = useState(starterCode);
  const [submissions, setSubmissions] = useState(initialSubmissions);
  const [selected, setSelected] = useState<Submission | null>(initialSubmissions[0] ?? null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setSubmissions(initialSubmissions);
    setSelected(initialSubmissions[0] ?? null);
  }, [initialSubmissions]);

  async function submit() {
    setError(null);
    const result = validateSubmissionSource(sourceCode);
    if (!result.ok) {
      setError(result.errors.join(" "));
      return;
    }
    setSubmitting(true);
    try {
      const created = await api.createSubmission(problemId, result.value);
      setSubmissions((current) => [created, ...current]);
      setSelected(created);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "提出できませんでした。");
    } finally {
      setSubmitting(false);
    }
  }

  async function refresh() {
    setError(null);
    try {
      const next = await api.listSubmissions(problemId);
      setSubmissions(next);
      if (selected) {
        setSelected(next.find((item) => item.id === selected.id) ?? next[0] ?? null);
      } else {
        setSelected(next[0] ?? null);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "提出履歴を取得できませんでした。");
    }
  }

  return (
    <section className="grid gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-bold tracking-normal">Submissions</h2>
        <button className="button-secondary" type="button" onClick={refresh}>
          <RefreshCcw size={17} />
          Refresh
        </button>
      </div>
      <ErrorMessage message={error} />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
        <div className="panel p-3">
          <label className="field">
            <span className="label">Python source</span>
            <textarea className="textarea min-h-[360px]" value={sourceCode} onChange={(event) => setSourceCode(event.target.value)} />
          </label>
          <div className="mt-3 flex justify-end">
            <button className="button-primary" type="button" onClick={submit} disabled={submitting}>
              <Play size={17} />
              {submitting ? "Submitting" : "Submit"}
            </button>
          </div>
        </div>
        <div className="grid content-start gap-3">
          {submissions.length === 0 ? (
            <EmptyState title="提出履歴はまだありません。" />
          ) : (
            submissions.map((submission) => (
              <button
                className={`panel grid gap-2 p-3 text-left hover:bg-panel ${selected?.id === submission.id ? "ring-2 ring-accent/20" : ""}`}
                key={submission.id}
                type="button"
                onClick={() => setSelected(submission)}
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="font-mono text-xs text-ink/55">#{submission.id}</span>
                  <StatusBadge value={submission.status} />
                </div>
                <div className="text-sm text-ink/60">{new Date(submission.created_at).toLocaleString()}</div>
                <div className="text-sm font-semibold">Score {submission.score}</div>
              </button>
            ))
          )}
        </div>
      </div>
      {selected ? (
        <div className="panel grid gap-3 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="font-semibold">Submission #{selected.id}</div>
            <StatusBadge value={selected.status} />
          </div>
          {selected.results.length === 0 ? (
            <div className="text-sm text-ink/60">Result rows are not available yet.</div>
          ) : (
            <div className="overflow-auto">
              <table className="w-full min-w-[680px] text-left text-sm">
                <thead className="bg-panel text-xs uppercase text-ink/55">
                  <tr>
                    <th className="px-3 py-2">Case</th>
                    <th className="px-3 py-2">Status</th>
                    <th className="px-3 py-2">Time</th>
                    <th className="px-3 py-2">Memory</th>
                    <th className="px-3 py-2">stderr</th>
                  </tr>
                </thead>
                <tbody>
                  {selected.results.map((result) => (
                    <tr className="border-t border-line" key={result.id}>
                      <td className="px-3 py-2 font-mono text-xs">{result.test_case_id}</td>
                      <td className="px-3 py-2">
                        <StatusBadge value={result.status} />
                      </td>
                      <td className="px-3 py-2">{result.execution_time_ms ?? "-"} ms</td>
                      <td className="px-3 py-2">{result.memory_kb ?? "-"} KB</td>
                      <td className="px-3 py-2 font-mono text-xs">{result.stderr || "-"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ) : null}
    </section>
  );
}
