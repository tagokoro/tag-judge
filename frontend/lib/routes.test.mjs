import { test } from "node:test";
import assert from "node:assert/strict";
import {
  libraryCodeEditPath,
  libraryEditPath,
  libraryPath,
  namespacePath,
  submissionUrlEditPath,
} from "./routes.ts";

test("page paths live under /n/ and encode each slug as one segment", () => {
  assert.equal(namespacePath("seed-author"), "/n/seed-author");
  assert.equal(libraryPath("seed-author", "union-find"), "/n/seed-author/union-find");
  assert.equal(namespacePath("a/b"), "/n/a%2Fb");
});

test("edit pages live beside the read-only pages", () => {
  assert.equal(libraryEditPath("ns", "uf"), "/n/ns/uf/edit");
  assert.equal(libraryCodeEditPath("ns", "uf"), "/n/ns/uf/code/edit");
  assert.equal(submissionUrlEditPath("ns", "uf"), "/n/ns/uf/submission-url/edit");
});
