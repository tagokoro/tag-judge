import { test } from "node:test";
import assert from "node:assert/strict";
import { libraryPath, namespacePath, parseVersionParam, versionPath } from "./routes.ts";

test("page paths live under /n/ and encode each slug as one segment", () => {
  assert.equal(namespacePath("seed-author"), "/n/seed-author");
  assert.equal(libraryPath("seed-author", "union-find"), "/n/seed-author/union-find");
  assert.equal(versionPath("seed-author", "union-find", 3), "/n/seed-author/union-find/v/3");
  assert.equal(namespacePath("a/b"), "/n/a%2Fb");
});

test("parseVersionParam accepts 1..int32 max only", () => {
  assert.equal(parseVersionParam("1"), 1);
  assert.equal(parseVersionParam("2147483647"), 2147483647);
  for (const raw of ["0", "-1", "01", "1.5", "abc", "", "2147483648"]) {
    assert.equal(parseVersionParam(raw), null, raw);
  }
});
