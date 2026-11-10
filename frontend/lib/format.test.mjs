import { test } from "node:test";
import assert from "node:assert/strict";
import { formatDateTime } from "./format.ts";

test("formatDateTime renders a readable date and keeps unparsable input", () => {
  assert.match(formatDateTime("2026-11-10T20:02:45.942103+09:00"), /^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}$/);
  assert.equal(formatDateTime("not a date"), "not a date");
});
