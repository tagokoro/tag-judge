import { test } from "node:test";
import assert from "node:assert/strict";
import {
  clearedSelectionData,
  deriveIdentity,
  effectiveVersion,
  emptyScopedData,
  initialWorkspaceState,
  WORKSPACE_DEFAULTS,
  workspaceReducer,
} from "./workspaceState.ts";
import {
  DEFAULT_LIBRARY_SLUG,
  DEFAULT_LIBRARY_SOURCE,
  DEFAULT_NAMESPACE_SLUG,
  DEFAULT_SUBMISSION_URL,
  SEED_USERS,
} from "./submissionUrl.ts";

// A DataGuard is captured at the START of an async call (a render / a fetch).
// It carries BOTH generations: the actor generation AND the selection
// generation. Keeping this in one helper means every test models the real
// guard the provider attaches.
function dataGuard(state) {
  return {
    authGeneration: state.authGeneration,
    selectionGeneration: state.selectionGeneration,
  };
}

function assertNullFields(state, fields, context) {
  for (const key of fields) {
    assert.equal(state.data[key], null, `${context}: ${key} was not cleared`);
  }
}

test("inlined workspace defaults stay in sync with lib/submissionUrl.ts", () => {
  assert.equal(WORKSPACE_DEFAULTS.namespaceSlug, DEFAULT_NAMESPACE_SLUG);
  assert.equal(WORKSPACE_DEFAULTS.librarySlug, DEFAULT_LIBRARY_SLUG);
  assert.equal(WORKSPACE_DEFAULTS.librarySource, DEFAULT_LIBRARY_SOURCE);
  assert.equal(WORKSPACE_DEFAULTS.submissionUrl, DEFAULT_SUBMISSION_URL);
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
  versionSource: {
    version_number: 1,
    source_code: "SECRET-SOURCE",
    submission_url: "https://example.com/submissions/9",
  },
  versionCreated: { version_number: 1, source_code: "SECRET-SOURCE" },
  submissionUrlInput: "https://example.com/submissions/secret",
  librarySourceInput: "SECRET-SOURCE-INPUT",
  patchVisibility: "public",
};

// Model the real UI: pick a version first (a target change), THEN load the
// data for that selection. SET_VERSION now clears version-scoped data, so the
// APPLY_DATA must come after it or the loaded source would be wiped.
function loadInto(state) {
  let next = workspaceReducer(state, { type: "SET_VERSION", version: 3 });
  next = workspaceReducer(next, {
    type: "APPLY_DATA",
    patch: SAMPLE,
    guard: dataGuard(next),
  });
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

// The full set of version/library data that must never survive a target change
// or a failed (404) Library detail refetch.
const VERSION_LIBRARY_FIELDS = [
  "libraryDetail",
  "versionSummaries",
  "versionSource",
  "versionCreated",
];
const VERSION_ONLY_FIELDS = ["versionSource"];

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
    guard: dataGuard(state),
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
  const staleGuard = dataGuard(state);
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
    guard: dataGuard(state),
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
    guard: dataGuard(state),
  });
  assert.equal(state.data.libraryDetail, null);
  assert.equal(state.data.versionSource, null);
  assert.equal(state.data.versionSummaries, null);
  assert.equal(state.data.versionCreated, null);
  // Surgical: unrelated identity-scoped values stay (clearing those is the
  // job of an actor change, not a failed detail refetch).
  assert.equal(state.data.submissionUrlInput, "https://example.com/submissions/secret");
});

test("applying an updated version replaces the displayed submission URL", () => {
  let state = loadedState();
  state = workspaceReducer(state, {
    type: "APPLY_DATA",
    patch: {
      versionSource: { version_number: 1, submission_url: "https://example.com/submissions/42" },
    },
    guard: dataGuard(state),
  });
  assert.equal(
    state.data.versionSource.submission_url,
    "https://example.com/submissions/42",
  );
});

test("ALL stale data writes from a previous actor are discarded (no private re-leak)", () => {
  let state = loadedState();
  const stale = dataGuard(state);
  // Actor changes while fetches / selections are in flight.
  state = workspaceReducer(state, { type: "SET_DEV_ENABLED", value: true });
  const clearedData = state.data;

  for (const action of [
    { type: "APPLY_DATA", patch: { libraryDetail: { id: "LEAK" } }, guard: stale },
    {
      type: "APPLY_DATA",
      patch: {
        versionSource: { version_number: 1, submission_url: "https://example.com/submissions/1" },
      },
      guard: stale,
    },
    { type: "CLEAR_LIBRARY_DETAIL", guard: stale },
    {
      type: "SET_LIBRARY_SLUG",
      slug: "late-lib",
      patch: { libraryDetail: { id: "LEAK" } },
      guard: stale,
    },
  ]) {
    const after = workspaceReducer(state, action);
    assert.equal(after.data, clearedData, `${action.type} from a stale actor must be ignored`);
  }
});

// ---------------------------------------------------------------------------
// PR #47 review regression suite (RED before the fix, GREEN after).
//
// Finding: CLEAR_LIBRARY_DETAIL and the SET_* selection actions never cleared
// the version/library data, and the async guard only tracked authGeneration,
// so an in-flight fetch for an OLD selection could write a private/foreign
// version (source and submission URL) into a NEW selection.
// ---------------------------------------------------------------------------

test("CLEAR_LIBRARY_DETAIL (404 after a private flip) also clears the version and its URL", () => {
  let state = loadedState();
  assert.ok(state.data.versionSource, "precondition: version loaded");
  state = workspaceReducer(state, {
    type: "CLEAR_LIBRARY_DETAIL",
    guard: dataGuard(state),
  });
  assertNullFields(state, VERSION_LIBRARY_FIELDS, "failed 404 detail refetch");
  // Surgical: unrelated identity-scoped values stay.
  assert.equal(state.data.submissionUrlInput, SAMPLE.submissionUrlInput);
  assert.deepEqual(state.data.libraries, SAMPLE.libraries);
  assert.deepEqual(state.data.namespaceResult, SAMPLE.namespaceResult);
});

test("SET_NAMESPACE_SLUG clears all version/library data", () => {
  let state = loadedState();
  state = workspaceReducer(state, { type: "SET_NAMESPACE_SLUG", slug: "other-ns" });
  assertNullFields(
    state,
    ["namespaceResult", "libraries", ...VERSION_LIBRARY_FIELDS],
    "namespace target change",
  );
  // Pure input widgets are not selection-scoped fetched data; they survive.
  assert.equal(state.data.submissionUrlInput, SAMPLE.submissionUrlInput);
  assert.equal(state.data.librarySourceInput, SAMPLE.librarySourceInput);
  assert.equal(state.data.patchVisibility, SAMPLE.patchVisibility);
});

test("SET_LIBRARY_SLUG clears version/library data but keeps the namespace list", () => {
  let state = loadedState();
  state = workspaceReducer(state, { type: "SET_LIBRARY_SLUG", slug: "other-lib" });
  assertNullFields(state, VERSION_LIBRARY_FIELDS, "library target change");
  assert.deepEqual(state.data.libraries, SAMPLE.libraries, "namespace-level list stays");
  assert.deepEqual(state.data.namespaceResult, SAMPLE.namespaceResult);
  assert.equal(state.data.submissionUrlInput, SAMPLE.submissionUrlInput);
});

test("SET_VERSION clears the version-scoped source/URL but keeps library-level data", () => {
  let state = loadedState();
  state = workspaceReducer(state, { type: "SET_VERSION", version: 4 });
  assertNullFields(state, VERSION_ONLY_FIELDS, "version target change");
  assert.deepEqual(state.data.libraryDetail, SAMPLE.libraryDetail);
  assert.deepEqual(state.data.versionSummaries, SAMPLE.versionSummaries);
  assert.deepEqual(state.data.versionCreated, SAMPLE.versionCreated);
  assert.equal(effectiveVersion(state), 4);
});

test("every target change bumps selectionGeneration; an actor change only bumps authGeneration", () => {
  let state = loadedState();
  let gen = state.selectionGeneration;
  for (const action of [
    { type: "SET_NAMESPACE_SLUG", slug: "x" },
    { type: "SET_LIBRARY_SLUG", slug: "y" },
    { type: "SET_VERSION", version: 9 },
    { type: "SET_VERSION", version: null },
    { type: "CLEAR_LIBRARY_DETAIL", guard: dataGuard(state) },
  ]) {
    // Refresh the guard for the actions that carry one (the CLEAR below).
    const guarded = action.guard ? { ...action, guard: dataGuard(state) } : action;
    state = workspaceReducer(state, guarded);
    assert.equal(
      state.selectionGeneration,
      gen + 1,
      `${action.type} must bump selectionGeneration`,
    );
    gen = state.selectionGeneration;
  }
  const auth = state.authGeneration;
  state = workspaceReducer(state, { type: "SET_DEV_ENABLED", value: true });
  assert.equal(state.selectionGeneration, gen, "actor change leaves selection gen alone");
  assert.equal(state.authGeneration, auth + 1, "actor change still bumps auth gen");
});

test("a late version fetch from a previous selection cannot restore a stale/foreign URL", () => {
  let state = loadedState();
  const staleGuard = dataGuard(state); // captured before the switch
  state = workspaceReducer(state, { type: "SET_LIBRARY_SLUG", slug: "other-lib" });
  // The in-flight getVersion / setSubmissionUrl for the OLD selection now
  // resolves with a (possibly private / foreign) version.
  state = workspaceReducer(state, {
    type: "APPLY_DATA",
    patch: {
      versionSource: { version_number: 1, submission_url: "https://old-private.example/p" },
    },
    guard: staleGuard,
  });
  assert.equal(state.data.versionSource, null, "stale version must not land");
  // A fetch for the CURRENT selection still applies.
  state = workspaceReducer(state, {
    type: "APPLY_DATA",
    patch: { versionSource: { version_number: 7, submission_url: "https://current.example/p" } },
    guard: dataGuard(state),
  });
  assert.equal(state.data.versionSource.version_number, 7);
});

test("selection generation (not a reused target identity) drops A->B->A late fetches", () => {
  const original = initialWorkspaceState.librarySlug;
  let state = loadedState();
  const staleGuard = dataGuard(state);
  state = workspaceReducer(state, { type: "SET_LIBRARY_SLUG", slug: "other-lib" });
  state = workspaceReducer(state, { type: "SET_LIBRARY_SLUG", slug: original }); // back to A
  state = workspaceReducer(state, {
    type: "APPLY_DATA",
    patch: { versionSource: { version_number: 1, submission_url: "https://stale.example/p" } },
    guard: staleGuard,
  });
  assert.equal(
    state.data.versionSource,
    null,
    "returning to the old target must not re-validate a stale in-flight fetch",
  );
});

test("a stale 404 clear from an old selection cannot wipe the new selection's data", () => {
  let state = loadedState();
  const staleGuard = dataGuard(state);
  state = workspaceReducer(state, { type: "SET_LIBRARY_SLUG", slug: "other-lib" });
  const fresh = {
    libraryDetail: { id: "fresh" },
    versionSource: { version_number: 2, submission_url: "https://fresh.example/p" },
  };
  state = workspaceReducer(state, {
    type: "APPLY_DATA",
    patch: fresh,
    guard: dataGuard(state),
  });
  // The OLD selection's detail fetch now 404s.
  const after = workspaceReducer(state, {
    type: "CLEAR_LIBRARY_DETAIL",
    guard: staleGuard,
  });
  assert.equal(after.data, state.data, "stale clear must be ignored");
  assert.deepEqual(after.data.versionSource, fresh.versionSource);
});

test("a 404 clear invalidates in-flight fetches for the failed target", () => {
  let state = loadedState();
  const inFlightGuard = dataGuard(state); // getVersion started before the 404
  state = workspaceReducer(state, {
    type: "CLEAR_LIBRARY_DETAIL",
    guard: dataGuard(state),
  });
  assert.equal(state.data.versionSource, null);
  // The in-flight getVersion resolves with the (now private) version.
  const after = workspaceReducer(state, {
    type: "APPLY_DATA",
    patch: { versionSource: { version_number: 1, submission_url: "https://private.example/p" } },
    guard: inFlightGuard,
  });
  assert.equal(after.data.versionSource, null, "late version fetch must not restore a leaked private URL");
});

test("atomic select+apply sets the new target's data and clears the old version", () => {
  let state = loadedState();
  const patch = {
    libraryDetail: { id: "new-lib", slug: "new", visibility: "public" },
    patchVisibility: "public",
    libraries: [{ id: "new-lib", visibility: "public" }],
  };
  state = workspaceReducer(state, {
    type: "SET_LIBRARY_SLUG",
    slug: "new",
    patch,
    guard: dataGuard(state),
  });
  assert.equal(state.librarySlug, "new");
  assert.deepEqual(state.data.libraryDetail, patch.libraryDetail);
  assert.deepEqual(state.data.libraries, patch.libraries);
  assert.equal(state.data.versionSource, null, "old version cleared for the new target");
});

test("atomic version select+apply replaces the previous version source", () => {
  let state = loadedState();
  const patch = {
    versionSource: { version_number: 7, source_code: "NEW-SOURCE" },
    versionCreated: { version_number: 7 },
    versionSummaries: [{ version_number: 7 }],
  };
  state = workspaceReducer(state, {
    type: "SET_VERSION",
    version: 7,
    patch,
    guard: dataGuard(state),
  });
  assert.equal(effectiveVersion(state), 7);
  assert.deepEqual(state.data.versionSource, patch.versionSource);
  assert.deepEqual(state.data.versionCreated, patch.versionCreated);
});

test("a stale atomic select+apply (late fetch after a switch) is dropped", () => {
  let state = loadedState();
  const staleGuard = dataGuard(state);
  state = workspaceReducer(state, { type: "SET_LIBRARY_SLUG", slug: "other-lib" });
  const after = workspaceReducer(state, {
    type: "SET_LIBRARY_SLUG",
    slug: "late-lib",
    patch: { libraryDetail: { id: "LATE" } },
    guard: staleGuard,
  });
  assert.equal(after.librarySlug, "other-lib", "stale selection must not override current");
  assert.equal(after.data, state.data, "stale patch must not land");
});

test("an actor change still discards a late atomic selection write (auth guard wins)", () => {
  let state = loadedState();
  const staleGuard = dataGuard(state);
  state = workspaceReducer(state, { type: "SET_DEV_ENABLED", value: true });
  const after = workspaceReducer(state, {
    type: "SET_VERSION",
    version: 9,
    patch: { versionSource: { id: "LEAK" } },
    guard: staleGuard,
  });
  assert.equal(after.data, state.data, "auth generation still wins over any patch");
});

test("clearedSelectionData encodes the exact per-scope field sets (inputs never cleared)", () => {
  const version = ["versionSource"];
  const library = [
    "libraryDetail",
    "versionCreated",
    "versionSource",
    "versionSummaries",
  ];
  const namespace = ["libraries", "namespaceResult", ...library];
  const expected = { version, library, namespace };
  const inputs = ["submissionUrlInput", "librarySourceInput", "patchVisibility"];
  for (const [scope, keys] of Object.entries(expected)) {
    const cleared = clearedSelectionData(scope);
    assert.deepEqual(Object.keys(cleared).sort(), [...keys].sort(), `${scope} scope keys`);
    for (const value of Object.values(cleared)) {
      assert.equal(value, null, `${scope}: every cleared field is null`);
    }
    for (const input of inputs) {
      assert.equal(input in cleared, false, `${scope} must not clear the ${input} input`);
    }
  }
});
