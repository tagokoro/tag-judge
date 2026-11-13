import { test } from "node:test";
import assert from "node:assert/strict";
import {
  libraryCodeEditPath,
  libraryEditPath,
  libraryPath,
  categoryPath,
  categoryEditPath,
  newLibraryPath,
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
