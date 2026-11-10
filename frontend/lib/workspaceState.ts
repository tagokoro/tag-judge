// Pure, framework-free state machine for the whole SPA. It owns EVERY
// identity-scoped value so that an actor/auth change clears private data in one
// place, and so late async results can be discarded when they arrive after the
// actor that requested them has changed.
//
// It ALSO owns selection-scoped data: everything fetched for one
// namespace/library/version target. Changing the target (or getting a 404 on a
// detail refetch) clears that data in one place, and a monotonic
// selectionGeneration (captured in each async guard) makes in-flight fetches for
// an OLD target un-restorable even if the user navigates A -> B -> A.
//
// No React here, and NO runtime imports from other local modules: node:test
// loads this file directly with Node's built-in TypeScript stripping (which
// requires extensionless relative imports to be type-only). The default values
// mirror lib/submissionUrl.ts; workspaceState.test.mjs asserts they stay in sync.

import type {
  Library,
  LibraryVersion,
  LibraryVersionSummary,
  Me,
  Namespace,
  Role,
  Visibility,
} from "./types";

export const WORKSPACE_DEFAULTS = {
  namespaceSlug: "seed-author",
  librarySlug: "union-find",
  librarySource: "def add(a, b):\n    return a + b\n",
  submissionUrl: "https://example.com/submissions/1",
  seedUsers: [
    { userId: "00000000-0000-4000-8000-000000000001", role: "author" as Role },
    { userId: "00000000-0000-4000-8000-000000000002", role: "admin" as Role },
  ],
} as const;

export interface DevUser {
  userId: string;
  role: Role;
}

/** A selected version is only meaningful for the library it was chosen under. */
export interface BoundVersion {
  namespaceSlug: string;
  librarySlug: string;
  version: number;
}

/** Everything that belongs to one actor / one library selection. */
export interface ScopedData {
  namespaceResult: Namespace | null;
  libraries: Library[] | null;
  libraryDetail: Library | null;
  versionSummaries: LibraryVersionSummary[] | null;
  // Displayed version: source_code (private) and its self-declared submission URL.
  versionSource: LibraryVersion | null;
  versionCreated: LibraryVersion | null;
  // Inputs / transient view state that could still carry a previous actor's text.
  submissionUrlInput: string;
  librarySourceInput: string;
  patchVisibility: Visibility;
}

/**
 * Captured at the START of an async call; the reducer drops stale results.
 *
 * authGeneration invalidates writes started under a previous ACTOR;
 * selectionGeneration invalidates writes started under a previous TARGET
 * (namespace/library/version, or a failed 404 detail refetch). A monotonic
 * counter (rather than a target tuple) also covers A -> B -> A round trips:
 * an in-flight fetch from the first visit to A can never be re-validated just
 * because the user navigated back.
 */
export interface DataGuard {
  authGeneration: number;
  selectionGeneration: number;
}

export interface WorkspaceState {
  // Authentication / actor
  session: Me | null;
  cookieChecked: boolean;
  devEnabled: boolean;
  devUser: DevUser;
  identity: string;
  authGeneration: number; // monotonic; bumped on EVERY identity change

  // Selection
  namespaceSlug: string;
  librarySlug: string;
  boundVersion: BoundVersion | null;
  refreshTick: number; // explicit "switch user" detail re-fetch (MVP §11.6)
  selectionGeneration: number; // monotonic; bumped on EVERY target change / 404

  data: ScopedData;
}

export function deriveIdentity(
  state: Pick<WorkspaceState, "session" | "devEnabled" | "devUser">,
): string {
  if (state.session) {
    return `cookie:${state.session.user_id}:${state.session.role}`;
  }
  if (state.devEnabled) {
    return `dev:${state.devUser.userId}:${state.devUser.role}`;
  }
  return "anon";
}

export function emptyScopedData(): ScopedData {
  return {
    namespaceResult: null,
    libraries: null,
    libraryDetail: null,
    versionSummaries: null,
    versionSource: null,
    versionCreated: null,
    submissionUrlInput: WORKSPACE_DEFAULTS.submissionUrl,
    librarySourceInput: WORKSPACE_DEFAULTS.librarySource,
    patchVisibility: "private",
  };
}

/**
 * Which scope of fetched data becomes stale on a target change / 404.
 * Inputs (submissionUrlInput, librarySourceInput, patchVisibility) are pure
 * transient widgets and are deliberately NOT cleared here: clearing them is the
 * job of an actor/auth change (emptyScopedData), not a selection change.
 */
export type SelectionScope = "namespace" | "library" | "version";

/** Data fetched for a specific VERSION (the displayed source and its URL). */
function clearedVersionData(): Partial<ScopedData> {
  return { versionSource: null };
}

/** Data fetched for a specific LIBRARY (detail, versions, source). */
function clearedLibraryData(): Partial<ScopedData> {
  return {
    libraryDetail: null,
    versionSummaries: null,
    versionSource: null,
    versionCreated: null,
  };
}

/** Everything fetched for a NAMESPACE (its profile + library list + the rest). */
function clearedNamespaceData(): Partial<ScopedData> {
  return { namespaceResult: null, libraries: null, ...clearedLibraryData() };
}

export function clearedSelectionData(scope: SelectionScope): Partial<ScopedData> {
  switch (scope) {
    case "namespace":
      return clearedNamespaceData();
    case "library":
      return clearedLibraryData();
    case "version":
      return clearedVersionData();
  }
}

export const initialWorkspaceState: WorkspaceState = {
  session: null,
  cookieChecked: false,
  devEnabled: false,
  devUser: {
    userId: WORKSPACE_DEFAULTS.seedUsers[0].userId,
    role: WORKSPACE_DEFAULTS.seedUsers[0].role,
  },
  identity: "anon",
  authGeneration: 0,
  namespaceSlug: WORKSPACE_DEFAULTS.namespaceSlug,
  librarySlug: WORKSPACE_DEFAULTS.librarySlug,
  boundVersion: null,
  refreshTick: 0,
  selectionGeneration: 0,
  data: emptyScopedData(),
};

/**
 * A selection action optionally carries a `patch` and a `guard`. When a fetch
 * both CHANGES the target (e.g. clicking a library/version in a list) and sets
 * data for the NEW target, it must do both ATOMICALLY: the reducer clears the
 * stale data, bumps selectionGeneration, and then applies the patch in one
 * step, so the fresh data is never wiped by the clear and a stale (guarded)
 * fetch can never override a newer selection.
 *
 * Pure navigation (typing a slug) carries no patch and no guard: it is a
 * synchronous user intent from the current render, always applied.
 *
 * CLEAR_LIBRARY_DETAIL / APPLY_DATA always carry a guard.
 */
type SelectionPatch = Partial<ScopedData>;

export type WorkspaceAction =
  | { type: "SESSION_COMMITTED"; session: Me | null }
  | { type: "SET_DEV_ENABLED"; value: boolean }
  | { type: "SET_DEV_USER_ID"; value: string }
  | { type: "SET_DEV_ROLE"; value: Role }
  | { type: "SWITCH_DEV_USER" }
  | {
      type: "SET_NAMESPACE_SLUG";
      slug: string;
      patch?: SelectionPatch;
      guard?: DataGuard;
    }
  | {
      type: "SET_LIBRARY_SLUG";
      slug: string;
      patch?: SelectionPatch;
      guard?: DataGuard;
    }
  | {
      type: "SET_VERSION";
      version: number | null;
      patch?: SelectionPatch;
      guard?: DataGuard;
    }
  | { type: "CLEAR_LIBRARY_DETAIL"; guard: DataGuard }
  | { type: "APPLY_DATA"; patch: SelectionPatch; guard: DataGuard };

/**
 * Recompute the effective actor. When it changes, drop ALL identity-scoped
 * data, reset the selected version, and bump authGeneration so in-flight
 * fetches (guarded by the previous generation) are discarded.
 * selectionGeneration is intentionally left alone: it is monotonic over the
 * whole session, and an actor change already invalidates every in-flight write
 * through authGeneration.
 */
function withIdentity(next: WorkspaceState, previousIdentity: string): WorkspaceState {
  const identity = deriveIdentity(next);
  if (identity === previousIdentity) {
    return { ...next, identity };
  }
  return {
    ...next,
    identity,
    authGeneration: next.authGeneration + 1,
    boundVersion: null,
    data: emptyScopedData(),
  };
}

function swappedDevUser(current: DevUser): DevUser {
  const [first, second] = WORKSPACE_DEFAULTS.seedUsers;
  return current.userId === first.userId
    ? { userId: second.userId, role: second.role }
    : { userId: first.userId, role: first.role };
}

/**
 * Apply a selection change atomically: bump selectionGeneration, clear the
 * data scoped to that level, then (optionally) apply the caller's patch for the
 * NEW target. Used by SET_NAMESPACE_SLUG / SET_LIBRARY_SLUG / SET_VERSION.
 */
function applySelectionChange(
  state: WorkspaceState,
  scope: SelectionScope,
  patch: SelectionPatch | undefined,
  selectionFields: Partial<WorkspaceState>,
): WorkspaceState {
  const cleared: ScopedData = { ...state.data, ...clearedSelectionData(scope) };
  return {
    ...state,
    ...selectionFields,
    selectionGeneration: state.selectionGeneration + 1,
    data: patch ? { ...cleared, ...patch } : cleared,
  };
}

export function workspaceReducer(
  state: WorkspaceState,
  action: WorkspaceAction,
): WorkspaceState {
  // EVERY action that writes data from an async result carries a DataGuard. A
  // late fetch or selection started under a previous actor OR a previous target
  // must not write private data back into the cleared state (privacy issue (1)
  // and the PR #47 selection race). Pure navigation (no guard) is always
  // applied because it is a synchronous user intent from the current render.
  if (
    "guard" in action &&
    action.guard &&
    (action.guard.authGeneration !== state.authGeneration ||
      action.guard.selectionGeneration !== state.selectionGeneration)
  ) {
    return state;
  }
  switch (action.type) {
    case "SESSION_COMMITTED": {
      const next: WorkspaceState = {
        ...state,
        session: action.session,
        cookieChecked: true,
        // A real cookie session always wins: force dev auth off.
        devEnabled: action.session ? false : state.devEnabled,
      };
      return withIdentity(next, state.identity);
    }
    case "SET_DEV_ENABLED":
      return withIdentity({ ...state, devEnabled: action.value }, state.identity);
    case "SET_DEV_USER_ID":
      return withIdentity(
        { ...state, devUser: { ...state.devUser, userId: action.value } },
        state.identity,
      );
    case "SET_DEV_ROLE":
      return withIdentity(
        { ...state, devUser: { ...state.devUser, role: action.value } },
        state.identity,
      );
    case "SWITCH_DEV_USER": {
      const next = withIdentity(
        { ...state, devUser: swappedDevUser(state.devUser) },
        state.identity,
      );
      // Re-run the current Library detail fetch so private -> 404 / public -> 200
      // is observable on the same page (MVP §11.6).
      return { ...next, refreshTick: next.refreshTick + 1 };
    }
    case "SET_NAMESPACE_SLUG":
      // A namespace change invalidates the namespace profile, its library list
      // and every library/version detail. Bound version is deliberately
      // NOT reset: effectiveVersion() binds it to the namespace/library it was
      // chosen under, so a slug change makes it unusable WITHOUT silently
      // reusing an old number (issue 2), and it becomes valid again if the user
      // navigates back to the original namespace.
      return applySelectionChange(state, "namespace", action.patch, {
        namespaceSlug: action.slug,
      });
    case "SET_LIBRARY_SLUG":
      // A library change invalidates the library detail, versions and source,
      // but keeps the namespace profile / library list (still valid for
      // the same namespace) and the input widgets.
      return applySelectionChange(state, "library", action.patch, {
        librarySlug: action.slug,
      });
    case "SET_VERSION":
      // A version change invalidates only version-scoped data (the displayed
      // source and its submission URL). Library-level data stays.
      return applySelectionChange(state, "version", action.patch, {
        boundVersion:
          action.version === null
            ? null
            : {
                namespaceSlug: state.namespaceSlug,
                librarySlug: state.librarySlug,
                version: action.version,
              },
      });
    case "CLEAR_LIBRARY_DETAIL":
      // A failed (e.g. 404 private) Library detail refetch must not leave the
      // previous detail or version source (with its submission URL) mounted for
      // a library the actor can no longer see. Bump selectionGeneration so any
      // in-flight version/detail fetch for the same (now-failed) target is also
      // discarded instead of restoring stale private/foreign data.
      return {
        ...state,
        selectionGeneration: state.selectionGeneration + 1,
        data: { ...state.data, ...clearedLibraryData() },
      };
    case "APPLY_DATA":
      return { ...state, data: { ...state.data, ...action.patch } };
    default:
      return state;
  }
}

/** Selected version is valid only for the namespace/library it was chosen for. */
export function effectiveVersion(state: WorkspaceState): number | null {
  const bound = state.boundVersion;
  if (!bound) {
    return null;
  }
  if (
    bound.namespaceSlug !== state.namespaceSlug ||
    bound.librarySlug !== state.librarySlug
  ) {
    return null;
  }
  return bound.version;
}
