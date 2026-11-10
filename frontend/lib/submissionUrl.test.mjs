import { test } from "node:test";
import assert from "node:assert/strict";
import * as submissionUrl from "./submissionUrl.ts";

const {
  isValidSubmissionUrl,
  MAX_SUBMISSION_URL_BYTES,
  DEFAULT_SUBMISSION_URL,
  DEFAULT_NAMESPACE_SLUG,
  DEFAULT_LIBRARY_SLUG,
  DEFAULT_LIBRARY_SOURCE,
  SEED_USERS,
} = submissionUrl;

test("UI defaults include a valid submission URL", () => {
  assert.equal(typeof DEFAULT_NAMESPACE_SLUG, "string");
  assert.equal(typeof DEFAULT_LIBRARY_SLUG, "string");
  assert.equal(typeof DEFAULT_LIBRARY_SOURCE, "string");
  assert.equal(typeof isValidSubmissionUrl, "function");
  assert.ok(isValidSubmissionUrl(DEFAULT_SUBMISSION_URL));
  assert.equal(SEED_USERS.length, 2);
});

test("isValidSubmissionUrl accepts absolute http/https URLs with a host", () => {
  const valid = [
    "https://example.com/submissions/1",
    "http://judge.example.org/a?b=c#frag",
    "https://例え.jp/submissions/1", // internationalized (Unicode) host
    "https://[::1]:8080/submissions/1", // IPv6 literal
    "https://user:pass@host.example/x", // userinfo with a real host
    "  https://example.com/submissions/2  ", // surrounding whitespace is trimmed
  ];
  for (const url of valid) {
    assert.equal(isValidSubmissionUrl(url), true, url);
  }
});

test("isValidSubmissionUrl rejects non-absolute / non-HTTP(S) / hostless URLs", () => {
  const invalid = [
    "",
    "   ",
    "example.com/submissions/1",
    "/submissions/1",
    "ftp://example.com/x",
    "javascript:alert(1)",
    "https:foo", // opaque scheme:foo form (no "//" authority)
    "https://?x",
    "https://@",
    "https://:443",
    "https://user@", // userinfo but no host
  ];
  for (const url of invalid) {
    assert.equal(isValidSubmissionUrl(url), false, url);
  }
});

test("isValidSubmissionUrl rejects control chars, internal whitespace and >2048 bytes", () => {
  assert.equal(MAX_SUBMISSION_URL_BYTES, 2048);
  assert.equal(isValidSubmissionUrl("https://example.com/a b"), false); // internal space
  assert.equal(isValidSubmissionUrl("https://example.com/\u0000"), false); // NUL control
  assert.equal(isValidSubmissionUrl("https://example.com/\u0007"), false); // BEL control
  assert.equal(isValidSubmissionUrl("https://example.com/\u0085"), false); // C1 control
  assert.equal(
    isValidSubmissionUrl(`https://example.com/${"a".repeat(2048)}`),
    false,
  );
  // "https://example.com/" is 20 bytes, so this stays within the 2048-byte cap.
  assert.equal(
    isValidSubmissionUrl(
      `https://example.com/${"a".repeat(MAX_SUBMISSION_URL_BYTES - 25)}`,
    ),
    true,
  );
});
