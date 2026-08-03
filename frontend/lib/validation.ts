import type { ProblemPayload, TestCasePayload, Visibility } from "@/lib/types";

export type ValidationResult<T> = {
  ok: true;
  value: T;
} | {
  ok: false;
  errors: string[];
};

export function validateProblemForm(input: Record<string, FormDataEntryValue | null>): ValidationResult<ProblemPayload> {
  const title = String(input.title ?? "").trim();
  const statement = String(input.statement ?? "");
  const timeLimit = Number(input.time_limit_ms);
  const memoryLimit = Number(input.memory_limit_kb);
  const visibility = String(input.visibility ?? "public") as Visibility;
  const errors: string[] = [];

  if (!title) errors.push("タイトルを入力してください。");
  if (!statement.trim()) errors.push("問題文を入力してください。");
  if (!Number.isInteger(timeLimit) || timeLimit <= 0) errors.push("実行時間制限は1以上の整数にしてください。");
  if (!Number.isInteger(memoryLimit) || memoryLimit <= 0) errors.push("メモリ制限は1以上の整数にしてください。");
  if (!["public", "private", "group"].includes(visibility)) errors.push("公開範囲が不正です。");

  if (errors.length > 0) {
    return { ok: false, errors };
  }
  return {
    ok: true,
    value: {
      title,
      statement,
      time_limit_ms: timeLimit,
      memory_limit_kb: memoryLimit,
      visibility
    }
  };
}

export function validateTestCaseForm(input: Record<string, FormDataEntryValue | null>): ValidationResult<TestCasePayload> {
  const score = Number(input.score);
  const errors: string[] = [];
  if (!Number.isInteger(score) || score <= 0) errors.push("スコアは1以上の整数にしてください。");
  if (errors.length > 0) {
    return { ok: false, errors };
  }
  return {
    ok: true,
    value: {
      input: String(input.input ?? ""),
      expected_output: String(input.expected_output ?? ""),
      is_sample: input.is_sample === "on",
      score
    }
  };
}

export function validateSubmissionSource(sourceCode: string): ValidationResult<string> {
  if (!sourceCode.trim()) {
    return { ok: false, errors: ["提出コードを入力してください。"] };
  }
  return { ok: true, value: sourceCode };
}
