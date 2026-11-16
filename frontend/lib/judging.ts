// Submission judging helpers: when to poll and what to tell the submitter.
// No React and no runtime imports from other local modules, so node:test can
// load this file directly.

/** Statuses that will still change; anything else is the final verdict. */
export function isJudging(status: string): boolean {
  return status === "pending" || status === "running";
}

const FIRST_POLL_MS = 500;
const MAX_POLL_MS = 3000;

/** Polling stops after this long and offers a manual reload instead. */
export const POLL_GIVE_UP_MS = 2 * 60 * 1000;

/** Consecutive failed polls after which polling stops and shows the error. */
export const MAX_POLL_FAILURES = 3;

/** The wait before poll number `attempt` (0-based): 0.5 s, growing 1.5x, capped at 3 s. */
export function pollDelay(attempt: number): number {
  return Math.min(MAX_POLL_MS, Math.round(FIRST_POLL_MS * 1.5 ** Math.max(0, attempt)));
}

/** The parts of an API error the messages below need. */
export interface ErrorLike {
  status?: number;
  code?: string;
  retryAfter?: number;
}

function later(retryAfter: number | undefined): string {
  return retryAfter ? `${retryAfter} 秒ほど待ってから` : "少し待ってから";
}

/**
 * What to show when a submission is refused, or null to fall back to the
 * generic error text. A missing problem gets no wording of its own: a hidden
 * one must read the same as one that does not exist.
 */
export function submitErrorMessage(error: ErrorLike): string | null {
  switch (error.code) {
    case "USER_RATE_LIMIT_EXCEEDED":
      return `提出が続いています。${later(error.retryAfter)}もう一度提出してください。`;
    case "USER_OUTSTANDING_LIMIT_EXCEEDED":
      return "判定中の提出があります。判定が終わってからもう一度提出してください。";
    case "GLOBAL_OUTSTANDING_LIMIT_EXCEEDED":
    case "TRANSIENT_DATABASE_ERROR":
      return `混み合っています。${later(error.retryAfter)}もう一度提出してください。`;
    case "PROBLEM_HAS_NO_TEST_CASES":
      return "この問題にはまだテストケースがないため、提出できません。";
    case "SOURCE_CODE_TOO_LARGE":
      return "コードが大きすぎます。64 KiB（65,536 バイト）までにしてください。";
    case "INVALID_SOURCE_CODE":
      return "コードを入力してください。";
    case "UNAUTHORIZED":
      return "提出するにはログインしてください。";
    default:
      return null;
  }
}

/** Source drafts are kept per problem in localStorage under this key. */
export function draftKey(problemId: number): string {
  return `tag-judge:draft:problem:${problemId}`;
}
