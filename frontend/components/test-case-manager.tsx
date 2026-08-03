"use client";

import { Plus, Save, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { EmptyState } from "@/components/empty-state";
import { ErrorMessage } from "@/components/error-message";
import { api, ApiError } from "@/lib/api";
import type { TestCase } from "@/lib/types";
import { validateTestCaseForm } from "@/lib/validation";

export function TestCaseManager({
  problemId,
  initialItems,
  onChange
}: {
  problemId: number;
  initialItems: TestCase[];
  onChange: () => void;
}) {
  const [items, setItems] = useState(initialItems);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setItems(initialItems);
  }, [initialItems]);

  async function save(event: React.FormEvent<HTMLFormElement>, testCase?: TestCase) {
    event.preventDefault();
    setError(null);
    const form = new FormData(event.currentTarget);
    const result = validateTestCaseForm({
      input: form.get("input"),
      expected_output: form.get("expected_output"),
      is_sample: form.get("is_sample"),
      score: form.get("score")
    });
    if (!result.ok) {
      setError(result.errors.join(" "));
      return;
    }
    try {
      const saved = testCase
        ? await api.updateTestCase(problemId, testCase.id, result.value)
        : await api.createTestCase(problemId, result.value);
      setItems((current) => (testCase ? current.map((item) => (item.id === saved.id ? saved : item)) : [...current, saved]));
      if (!testCase) {
        event.currentTarget.reset();
      }
      onChange();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "テストケースを保存できませんでした。");
    }
  }

  async function remove(testCase: TestCase) {
    setError(null);
    try {
      await api.deleteTestCase(problemId, testCase.id);
      setItems((current) => current.filter((item) => item.id !== testCase.id));
      onChange();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "テストケースを削除できませんでした。");
    }
  }

  return (
    <section className="grid gap-3">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-lg font-bold tracking-normal">Test Cases</h2>
        <span className="text-sm text-ink/55">{items.length} cases</span>
      </div>
      <ErrorMessage message={error} />
      <div className="grid gap-3">
        {items.length === 0 ? (
          <EmptyState title="テストケースはまだありません。" />
        ) : (
          items.map((testCase) => (
            <form className="panel grid gap-3 p-3" key={testCase.id} onSubmit={(event) => save(event, testCase)}>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="font-mono text-xs text-ink/55">#{testCase.id}</div>
                <label className="flex items-center gap-2 text-sm text-ink/70">
                  <input name="is_sample" type="checkbox" defaultChecked={testCase.is_sample} />
                  sample
                </label>
              </div>
              <div className="grid gap-3 lg:grid-cols-2">
                <label className="field">
                  <span className="label">Input</span>
                  <textarea className="textarea min-h-28" name="input" defaultValue={testCase.input} />
                </label>
                <label className="field">
                  <span className="label">Expected output</span>
                  <textarea className="textarea min-h-28" name="expected_output" defaultValue={testCase.expected_output} />
                </label>
              </div>
              <div className="flex flex-wrap items-end justify-between gap-3">
                <label className="field w-36">
                  <span className="label">Score</span>
                  <input className="input" name="score" type="number" min={1} defaultValue={testCase.score} />
                </label>
                <div className="flex gap-2">
                  <button className="button-secondary" type="submit">
                    <Save size={17} />
                    Save
                  </button>
                  <button className="button-danger" type="button" onClick={() => remove(testCase)} title="Delete">
                    <Trash2 size={17} />
                  </button>
                </div>
              </div>
            </form>
          ))
        )}
      </div>
      <form className="panel grid gap-3 bg-panel/60 p-3" onSubmit={(event) => save(event)}>
        <div className="font-semibold">Add case</div>
        <div className="grid gap-3 lg:grid-cols-2">
          <label className="field">
            <span className="label">Input</span>
            <textarea className="textarea min-h-24" name="input" />
          </label>
          <label className="field">
            <span className="label">Expected output</span>
            <textarea className="textarea min-h-24" name="expected_output" />
          </label>
        </div>
        <div className="flex flex-wrap items-end justify-between gap-3">
          <label className="flex items-center gap-2 text-sm text-ink/70">
            <input name="is_sample" type="checkbox" />
            sample
          </label>
          <label className="field w-36">
            <span className="label">Score</span>
            <input className="input" name="score" type="number" min={1} defaultValue={1} />
          </label>
          <button className="button-primary" type="submit">
            <Plus size={17} />
            Add
          </button>
        </div>
      </form>
    </section>
  );
}
