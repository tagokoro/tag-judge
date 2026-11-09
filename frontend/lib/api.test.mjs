import { test } from "node:test";
import assert from "node:assert/strict";
import {
  apiRequest,
  createJudgeClient,
  describeError,
  isApiRequestError,
  probeCookieSession,
} from "./api.ts";

function jsonResponse(status, body) {
  return {
    status,
    ok: status >= 200 && status < 300,
    text: async () => (body === undefined ? "" : JSON.stringify(body)),
  };
}

function recorder(response) {
  const calls = [];
  const fake = async (url, init) => {
    calls.push({ url, init });
    return typeof response === "function" ? response(url, init) : response;
  };
  return { calls, fake };
}

test("apiRequest unwraps {data} and always uses same-origin /api/v1", async () => {
  const { calls, fake } = recorder(jsonResponse(200, { data: { id: "abc" } }));
  const data = await apiRequest("/namespaces", { method: "GET" }, null, fake);
  assert.deepEqual(data, { id: "abc" });
  assert.equal(calls[0].url, "/api/v1/namespaces");
  assert.equal(calls[0].init.credentials, "same-origin");
  assert.equal(calls[0].init.headers["X-User-ID"], undefined);
});

test("apiRequest attaches dev headers only when explicitly provided", async () => {
  const { calls, fake } = recorder(jsonResponse(200, { data: 1 }));
  await apiRequest("/me", {}, null, fake);
  assert.equal(calls[0].init.headers["X-User-ID"], undefined);
  await apiRequest(
    "/me",
    {},
    { "X-User-ID": "u1", "X-User-Role": "admin" },
    fake,
  );
  assert.equal(calls[1].init.headers["X-User-ID"], "u1");
  assert.equal(calls[1].init.headers["X-User-Role"], "admin");
});

test("apiRequest JSON-encodes the body and sets Content-Type", async () => {
  const { calls, fake } = recorder(jsonResponse(201, { data: { ok: true } }));
  const input = { slug: "seed-author", display_name: "Seed Author" };
  await apiRequest("/namespaces", { method: "POST", json: input }, null, fake);
  assert.equal(calls[0].init.method, "POST");
  assert.equal(calls[0].init.headers["Content-Type"], "application/json");
  assert.deepEqual(JSON.parse(calls[0].init.body), input);
});

test("apiRequest throws a typed ApiRequestError on an error envelope", async () => {
  const { fake } = recorder(
    jsonResponse(404, {
      error: { code: "LIBRARY_NOT_FOUND", message: "library not found" },
    }),
  );
  await assert.rejects(
    () => apiRequest("/whatever", {}, null, fake),
    (error) => {
      assert.ok(isApiRequestError(error));
      assert.equal(error.code, "LIBRARY_NOT_FOUND");
      assert.equal(error.message, "library not found");
      assert.equal(error.status, 404);
      return true;
    },
  );
  assert.match(
    describeError(
      Object.assign(new Error("boom"), { name: "ApiRequestError", code: "X", status: 400 }),
    ),
    /X: boom \(HTTP 400\)/,
  );
});

test("apiRequest treats 204 (logout) as an empty success", async () => {
  const { calls, fake } = recorder({ status: 204, ok: true, text: async () => "" });
  const result = await apiRequest("/auth/logout", { method: "POST" }, null, fake);
  assert.equal(result, undefined);
  assert.equal(calls[0].url, "/api/v1/auth/logout");
});

test("probeCookieSession calls /me with no dev headers", async () => {
  const { calls, fake } = recorder(
    jsonResponse(200, { data: { user_id: "u", role: "author" } }),
  );
  const me = await probeCookieSession(fake);
  assert.deepEqual(me, { user_id: "u", role: "author" });
  assert.equal(calls[0].url, "/api/v1/me");
  assert.equal(calls[0].init.headers["X-User-ID"], undefined);
  assert.equal(calls[0].init.credentials, "same-origin");
});

test("createJudgeClient builds exact MVP endpoint paths and bodies", async () => {
  const { calls, fake } = recorder(jsonResponse(200, { data: {} }));
  const client = createJudgeClient(() => null, fake);

  await client.listProblems();
  assert.equal(calls[0].url, "/api/v1/problems?limit=100");

  await client.createLibrary("my-ns", {
    slug: "uf",
    name: "UnionFind",
    description: "",
    visibility: "private",
  });
  assert.equal(calls[1].url, "/api/v1/namespaces/my-ns/libraries");
  assert.equal(calls[1].init.method, "POST");
  assert.deepEqual(JSON.parse(calls[1].init.body), {
    slug: "uf",
    name: "UnionFind",
    description: "",
    visibility: "private",
  });

  await client.createVersion("my-ns", "uf", "def add(): pass\n");
  assert.equal(calls[2].url, "/api/v1/namespaces/my-ns/libraries/uf/versions");
  assert.deepEqual(JSON.parse(calls[2].init.body), {
    source_code: "def add(): pass\n",
  });

  await client.createVerification("my-ns", "uf", 3, {
    problem_id: 1,
    adapter_source: "print(add(1, 2))",
  });
  assert.equal(
    calls[3].url,
    "/api/v1/namespaces/my-ns/libraries/uf/versions/3/verifications",
  );

  await client.getVerification(7);
  assert.equal(calls[4].url, "/api/v1/library-verifications/7");

  await client.getNamespace("a b");
  assert.equal(calls[5].url, "/api/v1/namespaces/a%20b");
});

test("createJudgeClient uses the dev-header provider on every request", async () => {
  const { calls, fake } = recorder(jsonResponse(200, { data: {} }));
  const client = createJudgeClient(
    () => ({ "X-User-ID": "dev-user", "X-User-Role": "author" }),
    fake,
  );
  await client.getMe();
  assert.equal(calls[0].init.headers["X-User-ID"], "dev-user");
  assert.equal(calls[0].init.headers["X-User-Role"], "author");
});

test("client logout is a POST to the same-origin /auth/logout", async () => {
  const { calls, fake } = recorder({ status: 204, ok: true, text: async () => "" });
  const client = createJudgeClient(() => null, fake);
  const result = await client.logout();
  assert.equal(result, undefined);
  assert.equal(calls[0].url, "/api/v1/auth/logout");
  assert.equal(calls[0].init.method, "POST");
  assert.equal(calls[0].init.credentials, "same-origin");
});
