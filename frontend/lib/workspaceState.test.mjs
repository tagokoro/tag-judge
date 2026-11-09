import { test } from "node:test";
import assert from "node:assert/strict";
import {
  deriveIdentity,
  effectiveVersion,
  emptyScopedData,
  initialWorkspaceState,
  WORKSPACE_DEFAULTS,
  workspaceReducer,
} from "./workspaceState.ts";
import {
  DEFAULT_ADAPTER_SOURCE,
  DEFAULT_LIBRARY_SLUG,
  DEFAULT_LIBRARY_SOURCE,
  DEFAULT_NAMESPACE_SLUG,
  DEFAULT_PROBLEM_ID,
  SEED_USERS,
} from "./verification.ts";

test("inlined workspace defaults stay in sync with lib/verification.ts", () => {
  assert.equal(WORKSPACE_DEFAULTS.namespaceSlug, DEFAULT_NAMESPACE_SLUG);
  assert.equal(WORKSPACE_DEFAULTS.librarySlug, DEFAULT_LIBRARY_SLUG);
  assert.equal(WORKSPACE_DEFAULTS.librarySource, DEFAULT_LIBRARY_SOURCE);
  assert.equal(WORKSPACE_DEFAULTS.adapterSource, DEFAULT_ADAPTER_SOURCE);
  assert.equal(WORKSPACE_DEFAULTS.problemId, DEFAULT_PROBLEM_ID);
  assert.deepEqual(
    WORKSPACE_DEFAULTS.seedUsers.map((user) => ({ userId: user.userId, role: user.role })),
    SEED_USERS.map((user) => ({ userId: user.userId, role: user.role })),
  );
  assert.equal(initialWorkspaceState.namespaceSlug, DEFAULT_NAMESPACE_SLUG);
  assert.equal(initialWorkspaceState.librarySlug, DEFAULT_LIBRARY_SLUG);
});

// (Potentially private) data from "actor A" loaded into every identity-scoped field.
const SAMPLE = {
  namespaceResult: { id: "ns", slug: "seed-author" },
  libraries: [{ id: "lib", visibility: "private" }],
  libraryDetail: { id: "lib", slug: "union-find", visibility: "private" },
  versionSummaries: [{ version_number: 1 }],
  versionSource: { version_number: 1, source_code: "SECRET-SOURCE" },
  versionCreated: { version_number: 1, source_code: "SECRET-SOURCE" },
  problems: [{ id: 1, title: "A+B", statement: "secret statement" }],
  verifications: [{ id: 9, status: "ac" }],
  currentVerification: {
    id: 9,
    status: "ac",
    adapter_source: "SECRET-ADAPTER",
    case_results: [],
  },
  adapterSource: "SECRET-ADAPTER-INPUT",
  librarySourceInput: "SECRET-SOURCE-INPUT",
  problemId: 7,
  patchVisibility: "public",
};

function loadInto(state) {
  let next = workspaceReducer(state, {
    type: "APPLY_DATA",
    patch: SAMPLE,
    guard: { authGeneration: state.authGeneration },
  });
  next = workspaceReducer(next, { type: "SET_VERSION", version: 3 });
  return next;
}

function loadedState() {
  return loadInto(initialWorkspaceState);
}

function devLoadedState() {
  return loadInto(
    workspaceReducer(initialWorkspaceState, { type: "SET_DEV_ENABLED", value: true }),
  );
}

function cookieLoadedState() {
  return loadInto(
    workspaceReducer(initialWorkspaceState, {
      type: "SESSION_COMMITTED",
      session: { user_id: "u1", role: "author" },
    }),
  );
}

function assertScopedCleared(state) {
  const empty = emptyScopedData();
  for (const key of Object.keys(empty)) {
    assert.deepEqual(state.data[key], empty[key], `${key} was not cleared`);
  }
  assert.equal(state.boundVersion, null, "selected version was not reset");
}

test("deriveIdentity changes on every actor/auth transition and cookie wins", () => {
  const base = initialWorkspaceState;
  assert.equal(deriveIdentity(base), "anon");
  const devOn = { ...base, devEnabled: true };
  assert.equal(deriveIdentity(devOn), `dev:${SEED_USERS[0].userId}:author`);
  const devOther = {
    ...devOn,
    devUser: { userId: SEED_USERS[1].userId, role: "admin" },
  };
  assert.notEqual(deriveIdentity(devOn), deriveIdentity(devOther));
  const cookie = { ...base, session: { user_id: "u1", role: "admin" } };
  assert.equal(deriveIdentity(cookie), "cookie:u1:admin");
  assert.equal(
    deriveIdentity({ ...cookie, devEnabled: true }),
    "cookie:u1:admin",
    "a cookie session must suppress dev headers",
  );
});

test("every actor change clears all private data, bumps generation, resets version", () => {
  const cases = [
    {
      name: "cookie login",
      start: loadedState(),
      action: { type: "SESSION_COMMITTED", session: { user_id: "u2", role: "author" } },
    },
    {
      name: "dev auth enabled",
      start: loadedState(),
      action: { type: "SET_DEV_ENABLED", value: true },
    },
    {
      name: "dev user switched",
      start: devLoadedState(),
      action: { type: "SWITCH_DEV_USER" },
    },
    {
      name: "dev user id edited",
      start: devLoadedState(),
      action: { type: "SET_DEV_USER_ID", value: "another-user-id" },
    },
    {
      name: "dev role edited",
      start: devLoadedState(),
      action: { type: "SET_DEV_ROLE", value: "user" },
    },
    {
      name: "logout",
      start: cookieLoadedState(),
      action: { type: "SESSION_COMMITTED", session: null },
    },
  ];

  for (const { name, start, action } of cases) {
    assert.ok(start.data.libraryDetail, `${name}: precondition data loaded`);
    assert.equal(effectiveVersion(start), 3, `${name}: precondition version selected`);
    const after = workspaceReducer(start, action);
    assertScopedCleared(after);
    assert.equal(
      after.authGeneration,
      start.authGeneration + 1,
      `${name} must bump authGeneration`,
    );
  }
});

test("SESSION_COMMITTED forces dev auth off for a real cookie session", () => {
  let state = workspaceReducer(initialWorkspaceState, {
    type: "SET_DEV_ENABLED",
    value: true,
  });
  assert.equal(state.devEnabled, true);
  state = workspaceReducer(state, {
    type: "SESSION_COMMITTED",
    session: { user_id: "u1", role: "author" },
  });
  assert.equal(state.devEnabled, false);
});

test("re-probing the SAME session does not clear data or bump the generation", () => {
  let state = workspaceReducer(initialWorkspaceState, {
    type: "SESSION_COMMITTED",
    session: { user_id: "u1", role: "author" },
  });
  const generation = state.authGeneration;
  state = workspaceReducer(state, {
    type: "APPLY_DATA",
    patch: { namespaceResult: { id: "fresh" } },
    guard: { authGeneration: generation },
  });
  state = workspaceReducer(state, {
    type: "SESSION_COMMITTED",
    session: { user_id: "u1", role: "author" },
  });
  assert.equal(state.authGeneration, generation);
  assert.equal(state.data.namespaceResult.id, "fresh");
});

test("late async results from a previous actor are discarded", () => {
  let state = loadedState();
  const staleGuard = { authGeneration: state.authGeneration };
  // Actor changes while the fetch is in flight.
  state = workspaceReducer(state, { type: "SET_DEV_ENABLED", value: true });
  const cleared = state.data.libraryDetail;
  state = workspaceReducer(state, {
    type: "APPLY_DATA",
    patch: { libraryDetail: { id: "STALE", slug: "private", visibility: "private" } },
    guard: staleGuard,
  });
  assert.equal(state.data.libraryDetail, cleared, "stale fetch must not land");
  // A result from the CURRENT actor does apply.
  state = workspaceReducer(state, {
    type: "APPLY_DATA",
    patch: { namespaceResult: { id: "fresh" } },
    guard: { authGeneration: state.authGeneration },
  });
  assert.equal(state.data.namespaceResult.id, "fresh");
});

test("selected version is bound to namespace/library identity (no silent reuse)", () => {
  let state = workspaceReducer(initialWorkspaceState, {
    type: "SET_VERSION",
    version: 4,
  });
  assert.equal(effectiveVersion(state), 4);

  state = workspaceReducer(state, { type: "SET_NAMESPACE_SLUG", slug: "other-ns" });
  assert.equal(effectiveVersion(state), null, "version must not leak to a new namespace");

  state = workspaceReducer(state, {
    type: "SET_NAMESPACE_SLUG",
    slug: initialWorkspaceState.namespaceSlug,
  });
  assert.equal(effectiveVersion(state), 4, "valid again for the original namespace");

  state = workspaceReducer(state, { type: "SET_LIBRARY_SLUG", slug: "other-lib" });
  assert.equal(effectiveVersion(state), null, "version must not leak to a new library");
});

test("SET_VERSION binds to the current slugs and SET_VERSION null clears it", () => {
  let state = workspaceReducer(initialWorkspaceState, {
    type: "SET_LIBRARY_SLUG",
    slug: "lib-b",
  });
  state = workspaceReducer(state, { type: "SET_VERSION", version: 2 });
  assert.equal(state.boundVersion.librarySlug, "lib-b");
  assert.equal(effectiveVersion(state), 2);
  state = workspaceReducer(state, { type: "SET_VERSION", version: null });
  assert.equal(effectiveVersion(state), null);
});

test("actor change resets a bound version", () => {
  let state = workspaceReducer(initialWorkspaceState, {
    type: "SET_VERSION",
    version: 5,
  });
  assert.equal(effectiveVersion(state), 5);
  state = workspaceReducer(state, { type: "SET_DEV_ENABLED", value: true });
  assert.equal(state.boundVersion, null);
  assert.equal(effectiveVersion(state), null);
});

test("failed private Library detail refetch clears old detail and version source", () => {
  let state = loadedState();
  assert.ok(state.data.libraryDetail);
  assert.ok(state.data.versionSource);
  state = workspaceReducer(state, {
    type: "CLEAR_LIBRARY_DETAIL",
    guard: { authGeneration: state.authGeneration },
  });
  assert.equal(state.data.libraryDetail, null);
  assert.equal(state.data.versionSource, null);
  assert.equal(state.data.versionSummaries, null);
  assert.equal(state.data.versionCreated, null);
  // Surgical: unrelated identity-scoped values stay (clearing those is the
  // job of an actor change, not a failed detail refetch).
  assert.equal(state.data.adapterSource, "SECRET-ADAPTER-INPUT");
});

test("selecting a terminal verification replaces the current result", () => {
  let state = loadedState();
  state = workspaceReducer(state, {
    type: "SELECT_VERIFICATION",
    value: { id: 42, status: "ac", score: 100 },
    guard: { authGeneration: state.authGeneration },
  });
  assert.equal(state.data.currentVerification.id, 42);
  assert.equal(state.data.currentVerification.status, "ac");
});

test("ALL stale data writes from a previous actor are discarded (no private re-leak)", () => {
  let state = loadedState();
  const stale = { authGeneration: state.authGeneration };
  // Actor changes while fetches / selections are in flight.
  state = workspaceReducer(state, { type: "SET_DEV_ENABLED", value: true });
  const clearedData = state.data;

  for (const action of [
    { type: "APPLY_DATA", patch: { libraryDetail: { id: "LEAK" } }, guard: stale },
    { type: "SELECT_VERIFICATION", value: { id: 1, status: "ac" }, guard: stale },
    { type: "CLEAR_LIBRARY_DETAIL", guard: stale },
  ]) {
    const after = workspaceReducer(state, action);
    assert.equal(after.data, clearedData, `${action.type} from a stale actor must be ignored`);
  }
});
