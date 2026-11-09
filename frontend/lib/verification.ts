// Pure view-model helpers for the verification flow. Kept free of React and of
// runtime imports from other local modules so it can be unit-tested directly
// with Node's built-in test runner (node --test *.test.mjs).

import type { CaseResult, Role, VerificationDetail, VerificationSummary } from "./types";

export const TERMINAL_STATUSES = [
  "ac",
  "wa",
  "tle",
  "mle",
  "re",
  "ce",
  "ie",
] as const;

export function isTerminalStatus(status: string): boolean {
  return (TERMINAL_STATUSES as readonly string[]).includes(status);
}

export const STATUS_LABELS: Record<string, string> = {
  pending: "pending (待機)",
  running: "running (実行中)",
  ac: "AC (正解)",
  wa: "WA (不正解)",
  tle: "TLE (時間超過)",
  mle: "MLE (メモリ超過)",
  re: "RE (実行時エラー)",
  ce: "CE (コンパイルエラー)",
  ie: "IE (内部エラー)",
};

export function statusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status;
}

export function formatCaseResult(result: CaseResult): string {
  const time = result.execution_time_ms === null ? "-" : `${result.execution_time_ms} ms`;
  const memory = result.memory_kb === null ? "-" : `${result.memory_kb} KiB`;
  return `#${result.test_case_id} ${statusLabel(result.status)} · ${time} · ${memory}`;
}

export function isVerificationDetail(
  value: VerificationSummary | VerificationDetail,
): value is VerificationDetail {
  return Array.isArray((value as VerificationDetail).case_results);
}

export function caseResultsOf(
  value: VerificationSummary | VerificationDetail,
): CaseResult[] {
  return isVerificationDetail(value) ? value.case_results : [];
}

export const SEED_USERS = [
  { userId: "00000000-0000-4000-8000-000000000001", role: "author" as Role },
  { userId: "00000000-0000-4000-8000-000000000002", role: "admin" as Role },
] as const;

export const DEFAULT_NAMESPACE_SLUG = "seed-author";
export const DEFAULT_LIBRARY_SLUG = "union-find";
export const DEFAULT_LIBRARY_SOURCE = "def add(a, b):\n    return a + b\n";
export const DEFAULT_ADAPTER_SOURCE =
  "a, b = map(int, input().split())\nprint(add(a, b))\n";
export const DEFAULT_PROBLEM_ID = 1;

export interface PollOptions<T> {
  intervalMs?: number;
  maxAttempts?: number;
  sleep?: (ms: number) => Promise<void>;
  onUpdate?: (value: T) => void;
  shouldContinue?: () => boolean;
}

const defaultSleep = (ms: number): Promise<void> =>
  new Promise<void>((resolve) => {
    setTimeout(resolve, ms);
  });

/**
 * Poll a verification detail until a terminal status is reached. The agreed MVP
 * contract: 1 second interval, at most 30 GETs. `shouldContinue` lets callers
 * cancel (e.g. on component unmount). `onUpdate` receives every fetched value.
 */
export async function pollVerification<T extends { status: string }>(
  fetchDetail: () => Promise<T>,
  initial: T,
  options: PollOptions<T> = {},
): Promise<T> {
  const {
    intervalMs = 1000,
    maxAttempts = 30,
    sleep = defaultSleep,
    onUpdate,
    shouldContinue,
  } = options;

  let current = initial;
  onUpdate?.(current);

  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    if (isTerminalStatus(current.status)) {
      return current;
    }
    if (shouldContinue && !shouldContinue()) {
      return current;
    }
    await sleep(intervalMs);
    if (shouldContinue && !shouldContinue()) {
      return current;
    }
    current = await fetchDetail();
    onUpdate?.(current);
  }

  return current;
}
