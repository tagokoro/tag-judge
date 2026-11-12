import { test } from "node:test";
import assert from "node:assert/strict";
import { formatDateTime } from "./format.ts";

test("formatDateTime renders a readable date and keeps unparsable input", () => {
  assert.match(formatDateTime("2026-11-10T20:02:45.942103+09:00"), /^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}$/);
  assert.equal(formatDateTime("not a date"), "not a date");
});

test("statusLabel and statusTone describe judge results", async () => {
  const { statusLabel, statusTone } = await import("./format.ts");
  assert.equal(statusLabel("ac"), "AC");
  assert.equal(statusLabel("pending"), "待機中");
  assert.equal(statusLabel("unknown"), "unknown");
  assert.equal(statusTone("ac"), "accepted");
  assert.equal(statusTone("wa"), "rejected");
  assert.equal(statusTone("running"), "waiting");
});

test("descriptionSummary keeps the first prose paragraph as plain text", async () => {
  const { descriptionSummary } = await import("./format.ts");
  const seed =
    "素集合を管理する **Union-Find** です。\n\n- `find(x)`: 根を返す\n\n計算量はならし $O(\\alpha(N))$ です。\n\n$$\\alpha(N) \\le 4$$";
  assert.equal(descriptionSummary(seed), "素集合を管理する Union-Find です。");
  assert.equal(descriptionSummary("# 見出し\n\n[解説](https://example.com) と `code`"), "解説 と code");
  assert.equal(descriptionSummary("$$x$$\n\n- a\n- b"), "");
});
