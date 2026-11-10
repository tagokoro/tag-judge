// Pure view-model helpers and shared defaults for the library-sharing UI. Kept
// free of React and of runtime imports from other local modules so Node's
// built-in test runner (node:test) can load it directly with TypeScript
// stripping.

import type { Role } from "./types";

/** Mirrors the backend/DB cap (domain.MaxSubmissionURLLength). */
export const MAX_SUBMISSION_URL_BYTES = 2048;

/**
 * Client-side mirror of the backend's AUTHORITATIVE submission-URL format gate
 * (backend/internal/domain ValidateSubmissionURL). The backend always has the
 * final say (invalid URLs get 400); this helper only gives immediate feedback.
 *
 * It trims surrounding whitespace and requires an absolute http/https URL with
 * a non-empty host, no control characters or internal whitespace, and at most
 * MAX_SUBMISSION_URL_BYTES bytes. It never fetches or judges the URL.
 */
export function isValidSubmissionUrl(raw: string): boolean {
  const value = raw.trim();
  // Require an explicit "//" authority so opaque "scheme:foo" forms are
  // rejected (the backend rejects non-empty Opaque). Case-insensitive scheme.
  if (!/^https?:\/\//i.test(value)) {
    return false;
  }
  if (utf8Length(value) > MAX_SUBMISSION_URL_BYTES) {
    return false;
  }
  for (const char of value) {
    const code = char.codePointAt(0) ?? 0;
    // C0/DEL and C1 control characters, plus any Unicode whitespace.
    if (code <= 0x1f || (code >= 0x7f && code <= 0x9f)) {
      return false;
    }
    if (/\s/u.test(char)) {
      return false;
    }
  }
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    return false;
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return false;
  }
  // Hostname() is non-empty for every real authority; hostless forms such as
  // "https://?x", "https://@", "https://:443" and "https://user@" throw above.
  return parsed.hostname !== "";
}

function utf8Length(value: string): number {
  return new TextEncoder().encode(value).length;
}

export const SEED_USERS = [
  { userId: "00000000-0000-4000-8000-000000000001", role: "author" as Role },
  { userId: "00000000-0000-4000-8000-000000000002", role: "admin" as Role },
] as const;

export const DEFAULT_NAMESPACE_SLUG = "seed-author";
export const DEFAULT_LIBRARY_SLUG = "union-find";
export const DEFAULT_LIBRARY_SOURCE = "def add(a, b):\n    return a + b\n";
export const DEFAULT_SUBMISSION_URL = "https://example.com/submissions/1";
