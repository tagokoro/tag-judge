import { test } from "node:test";
import assert from "node:assert/strict";
import {
  caseResultsOf,
  formatCaseResult,
  isPollTokenCurrent,
  isTerminalStatus,
  isVerificationDetail,
  nextPollToken,
  pollVerification,
  statusLabel,
} from "./verification.ts";

const noSleep = async () => {};

test("isTerminalStatus recognises all terminal verdicts", () => {
  for (const status of ["ac", "wa", "tle", "mle", "re", "ce", "ie"]) {
    assert.equal(isTerminalStatus(status), true, status);
  }
  for (const status of ["pending", "running", "unknown"]) {
    assert.equal(isTerminalStatus(status), false, status);
  }
});

test("statusLabel and formatCaseResult format null metrics safely", () => {
  assert.equal(statusLabel("ac"), "AC (正解)");
  assert.equal(statusLabel("weird"), "weird");
  assert.equal(
    formatCaseResult({
      test_case_id: 2,
      status: "ac",
      execution_time_ms: 12,
      memory_kb: null,
    }),
    "#2 AC (正解) · 12 ms · -",
  );
});

test("caseResultsOf only exposes case_results from a detail response", () => {
  const summary = { id: 1, status: "pending" };
  assert.equal(isVerificationDetail(summary), false);
  assert.deepEqual(caseResultsOf(summary), []);
  const detail = {
    id: 1,
    status: "ac",
    adapter_source: "print(1)",
    case_results: [{ test_case_id: 1, status: "ac", execution_time_ms: 3, memory_kb: null }],
  };
  assert.equal(isVerificationDetail(detail), true);
  assert.equal(caseResultsOf(detail).length, 1);
});

test("pollVerification does not fetch when the initial status is terminal", async () => {
  let fetches = 0;
  const updates = [];
  const initial = { status: "ac", score: 100 };
  const result = await pollVerification(
    async () => {
      fetches += 1;
      return { status: "ac" };
    },
    initial,
    { sleep: noSleep, onUpdate: (value) => updates.push(value.status) },
  );
  assert.equal(fetches, 0);
  assert.equal(result, initial);
  assert.deepEqual(updates, ["ac"]);
});

test("pollVerification follows pending -> running -> ac and reports updates", async () => {
  const sequence = [
    { status: "running", score: 0 },
    { status: "ac", score: 100, case_results: [] },
  ];
  let fetches = 0;
  const updates = [];
  const result = await pollVerification(
    async () => {
      const value = sequence[fetches];
      fetches += 1;
      return value;
    },
    { status: "pending", score: 0 },
    {
      sleep: noSleep,
      onUpdate: (value) => updates.push(value.status),
    },
  );
  assert.equal(fetches, 2);
  assert.equal(result.status, "ac");
  assert.deepEqual(updates, ["pending", "running", "ac"]);
});

test("pollVerification stops after maxAttempts GETs", async () => {
  let fetches = 0;
  const result = await pollVerification(
    async () => {
      fetches += 1;
      return { status: "running" };
    },
    { status: "pending" },
    { sleep: noSleep, maxAttempts: 30 },
  );
  assert.equal(fetches, 30);
  assert.equal(result.status, "running");
});

test("pollVerification honours shouldContinue cancellation", async () => {
  let fetches = 0;
  let allow = true;
  await pollVerification(
    async () => {
      fetches += 1;
      return { status: "running" };
    },
    { status: "pending" },
    {
      sleep: async () => {
        allow = false;
      },
      shouldContinue: () => allow,
      maxAttempts: 10,
    },
  );
  assert.equal(fetches, 0);
});

test("nextPollToken / isPollTokenCurrent detect a superseded selection", () => {
  const token = 5;
  const current = nextPollToken(token);
  assert.equal(isPollTokenCurrent(token, current), false);
  assert.equal(isPollTokenCurrent(current, current), true);
});

test("bumping the token mid-poll stops it and prevents a stale overwrite", async () => {
  const holder = { token: 3 };
  let fetches = 0;
  const updates = [];
  const result = await pollVerification(
    async () => {
      fetches += 1;
      return { status: "running", id: 1 };
    },
    { status: "pending", id: 1 },
    {
      sleep: async () => {
        // Simulate the user selecting ANOTHER verification (terminal or not)
        // while the poll is waiting between GETs.
        holder.token = nextPollToken(holder.token);
      },
      shouldContinue: () => isPollTokenCurrent(3, holder.token),
      onUpdate: (value) => updates.push(value.status),
      maxAttempts: 5,
    },
  );
  assert.equal(fetches, 0, "no detail GET should run after invalidation");
  assert.equal(result.status, "pending");
  assert.deepEqual(updates, ["pending"], "only the initial update; no late poll overwrite");
});

test("selecting a TERMINAL result still invalidates an in-flight poll", async () => {
  const holder = { token: 1 };
  let onUpdateCalls = 0;
  const result = await pollVerification(
    async () => ({ status: "ac", id: 2 }),
    { status: "pending", id: 2 },
    {
      sleep: async () => {
        // A terminal selection also bumps the token, so the running poll must
        // not deliver its late `ac` value over the user's chosen result.
        holder.token = nextPollToken(holder.token);
      },
      shouldContinue: () => isPollTokenCurrent(1, holder.token),
      onUpdate: () => {
        onUpdateCalls += 1;
      },
      maxAttempts: 3,
    },
  );
  assert.equal(result.status, "pending");
  assert.equal(onUpdateCalls, 1, "only the initial update; stale poll cannot overwrite");
});
