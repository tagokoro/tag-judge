import { test } from "node:test";
import assert from "node:assert/strict";
import { draftKey, isJudging, MAX_POLL_FAILURES, pollDelay, POLL_GIVE_UP_MS, submitErrorMessage } from "./judging.ts";

test("only pending and running are still being judged", () => {
  assert.equal(isJudging("pending"), true);
  assert.equal(isJudging("running"), true);
  for (const status of ["ac", "wa", "tle", "mle", "re", "ce", "ie"]) {
    assert.equal(isJudging(status), false, status);
  }
});

test("polling starts at 0.5 s, grows 1.5x, and caps at 3 s", () => {
  assert.deepEqual(
    [0, 1, 2, 3, 4, 5, 10].map(pollDelay),
    [500, 750, 1125, 1688, 2531, 3000, 3000],
  );
  assert.equal(pollDelay(-1), 500);
  assert.equal(POLL_GIVE_UP_MS, 120000);
  assert.equal(MAX_POLL_FAILURES, 3);
});

test("refused submissions get a message, using Retry-After when sent", () => {
  assert.match(submitErrorMessage({ code: "USER_RATE_LIMIT_EXCEEDED", retryAfter: 12 }), /12 秒ほど待ってから/);
  assert.match(submitErrorMessage({ code: "USER_RATE_LIMIT_EXCEEDED" }), /少し待ってから/);
  assert.match(submitErrorMessage({ code: "USER_OUTSTANDING_LIMIT_EXCEEDED" }), /判定中の提出があります/);
  assert.match(submitErrorMessage({ code: "GLOBAL_OUTSTANDING_LIMIT_EXCEEDED", retryAfter: 5 }), /混み合っています。5 秒/);
  assert.match(submitErrorMessage({ code: "PROBLEM_HAS_NO_TEST_CASES" }), /テストケースがない/);
  assert.match(submitErrorMessage({ code: "SOURCE_CODE_TOO_LARGE" }), /64 KiB/);
});

// A hidden draft must read like a missing problem, so 404 has no wording of its own.
test("a missing problem falls back to the generic error", () => {
  assert.equal(submitErrorMessage({ status: 404, code: "PROBLEM_NOT_FOUND" }), null);
  assert.equal(submitErrorMessage({ status: 500, code: "INTERNAL_ERROR" }), null);
});

test("drafts are kept per problem", () => {
  assert.equal(draftKey(42), "tag-judge:draft:problem:42");
  assert.notEqual(draftKey(1), draftKey(2));
});
