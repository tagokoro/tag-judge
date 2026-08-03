"use client";

import { Save } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { Problem } from "@/lib/types";
import { validateProblemForm } from "@/lib/validation";
import { ErrorMessage } from "@/components/error-message";

export function ProblemForm({ problem }: { problem?: Problem }) {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    const form = new FormData(event.currentTarget);
    const result = validateProblemForm({
      title: form.get("title"),
      statement: form.get("statement"),
      time_limit_ms: form.get("time_limit_ms"),
      memory_limit_kb: form.get("memory_limit_kb"),
      visibility: form.get("visibility")
    });
    if (!result.ok) {
      setError(result.errors.join(" "));
      return;
    }

    setSaving(true);
    try {
      const saved = problem ? await api.updateProblem(problem.id, result.value) : await api.createProblem(result.value);
      router.push(`/problems/${saved.id}`);
      router.refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存に失敗しました。");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="grid gap-4" onSubmit={submit}>
      <ErrorMessage message={error} />
      <div className="grid gap-4 lg:grid-cols-[1fr_180px_180px_180px]">
        <label className="field">
          <span className="label">Title</span>
          <input className="input" name="title" defaultValue={problem?.title ?? ""} required maxLength={160} />
        </label>
        <label className="field">
          <span className="label">Time ms</span>
          <input className="input" name="time_limit_ms" type="number" min={1} defaultValue={problem?.time_limit_ms ?? 2000} required />
        </label>
        <label className="field">
          <span className="label">Memory KB</span>
          <input className="input" name="memory_limit_kb" type="number" min={1} defaultValue={problem?.memory_limit_kb ?? 262144} required />
        </label>
        <label className="field">
          <span className="label">Visibility</span>
          <select className="input" name="visibility" defaultValue={problem?.visibility ?? "public"}>
            <option value="public">public</option>
            <option value="private">private</option>
            <option value="group">group</option>
          </select>
        </label>
      </div>
      <label className="field">
        <span className="label">Statement</span>
        <textarea className="textarea min-h-[420px]" name="statement" defaultValue={problem?.statement ?? ""} required />
      </label>
      <div className="flex justify-end">
        <button className="button-primary" type="submit" disabled={saving}>
          <Save size={17} />
          {saving ? "Saving" : "Save"}
        </button>
      </div>
    </form>
  );
}
