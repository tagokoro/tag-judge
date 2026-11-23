import { test } from "node:test";
import assert from "node:assert/strict";
import {
  libraryCodeEditPath,
  libraryEditPath,
  libraryPath,
  categoryPath,
  categoryEditPath,
  newLibraryPath,
  parseIdSegment,
  problemPath,
  signupNextPath,
  submissionPath,
  submissionUrlEditPath,
  userPath,
} from "./routes.ts";

const ns = "aaaaaaaaaaaaaaaaaaaa";
const lib = "bbbbbbbbbbbbbbbbbbbb";

test("pages nest under /users/{handle} and find resources by id", () => {
  assert.equal(userPath("alice"), "/users/alice");
  assert.equal(categoryPath("alice", ns), `/users/alice/categories/${ns}`);
  assert.equal(libraryPath("alice", lib), `/users/alice/libraries/${lib}`);
  assert.equal(userPath("a/b"), "/users/a%2Fb");
});

test("edit pages live beside the read-only pages", () => {
  assert.equal(libraryEditPath("alice", lib), `/users/alice/libraries/${lib}/edit`);
  assert.equal(libraryCodeEditPath("alice", lib), `/users/alice/libraries/${lib}/code/edit`);
  assert.equal(submissionUrlEditPath("alice", lib), `/users/alice/libraries/${lib}/submission-url/edit`);
});

test("category editing and library creation take the category id", () => {
  assert.equal(categoryEditPath("alice", ns), `/users/alice/categories/${ns}/edit`);
  assert.equal(newLibraryPath(), "/libraries/new");
  assert.equal(newLibraryPath(ns), `/libraries/new?category=${ns}`);
});

test("problems and submissions are found by their numeric id", () => {
  assert.equal(problemPath(3), "/problems/3");
  assert.equal(problemPath(3, "submit"), "/problems/3?tab=submit");
  assert.equal(submissionPath(10), "/submissions/10");
});

test("only a positive integer segment names a problem or submission", () => {
  assert.equal(parseIdSegment("42"), 42);
  for (const segment of ["0", "-1", "01", "1.5", "abc", "", "1e3", "12345678901234567"]) {
    assert.equal(parseIdSegment(segment), null, segment);
  }
});

test("sign-up continues only to the OAuth consent page", () => {
  assert.equal(
    signupNextPath("?next=" + encodeURIComponent("/oauth/authorize?client_id=a&state=b")),
    "/oauth/authorize?client_id=a&state=b",
  );
  assert.equal(signupNextPath(""), null);
  assert.equal(signupNextPath("?next=" + encodeURIComponent("https://evil.example/oauth/authorize?")), null);
  assert.equal(signupNextPath("?next=" + encodeURIComponent("//evil.example/oauth/authorize?")), null);
});
