// Pure, framework-free state machine for the whole SPA. It owns EVERY
// identity-scoped value so that an actor/auth change clears private data in one
// place, and so late async results can be discarded when they arrive after the
// actor that requested them has changed.
//
// No React here, and NO runtime imports from other local modules: node:test
// loads this file directly with Node's built-in TypeScript stripping (which
// requires extensionless relative imports to be type-only). The default values
// mirror lib/verification.ts; workspaceState.test.mjs asserts they stay in sync.

import type {
  Library,
  LibraryVersion,
  LibraryVersionSummary,
  Me,
  Namespace,
  Role,
  VerificationLink,
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
  versionSource: LibraryVersion | null; // displayed source_code (private)
  versionCreated: LibraryVersion | null;
  // Self-declared related-submission URL links for the selected version.
  links: VerificationLink[] | null;
  currentLink: VerificationLink | null; // selected link detail
  // Inputs / transient view state that could still carry a previous actor's text.
  submissionUrlInput: string;
  librarySourceInput: string;
  patchVisibility: Visibility;
}

/** Captured at the start of an async call; the reducer drops stale results. */
export interface DataGuard {
  authGeneration: number;
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
    links: null,
    currentLink: null,
    submissionUrlInput: WORKSPACE_DEFAULTS.submissionUrl,
    librarySourceInput: WORKSPACE_DEFAULTS.librarySource,
    patchVisibility: "private",
  };
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
  data: emptyScopedData(),
};

export type WorkspaceAction =
  | { type: "SESSION_COMMITTED"; session: Me | null }
  | { type: "SET_DEV_ENABLED"; value: boolean }
  | { type: "SET_DEV_USER_ID"; value: string }
  | { type: "SET_DEV_ROLE"; value: Role }
  | { type: "SWITCH_DEV_USER" }
  | { type: "SET_NAMESPACE_SLUG"; slug: string }
  | { type: "SET_LIBRARY_SLUG"; slug: string }
  | { type: "SET_VERSION"; version: number | null }
  | { type: "CLEAR_LIBRARY_DETAIL"; guard: DataGuard }
  | { type: "APPLY_DATA"; patch: Partial<ScopedData>; guard: DataGuard };

/**
 * Recompute the effective actor. When it changes, drop ALL identity-scoped
 * data, reset the selected version, and bump authGeneration so in-flight
 * fetches (guarded by the previous generation) are discarded.
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

export function workspaceReducer(
  state: WorkspaceState,
  action: WorkspaceAction,
): WorkspaceState {
  // EVERY action that writes identity-scoped data carries a DataGuard. A late
  // fetch or selection started under a previous actor must not write private
  // data back into the cleared state (privacy issue (1)).
  if ("guard" in action && action.guard.authGeneration !== state.authGeneration) {
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
      // Do NOT reset boundVersion here: effectiveVersion() binds it to the
      // namespace/library it was chosen under, so a slug change makes it
      // unusable WITHOUT silently reusing an old number (issue 2).
      return { ...state, namespaceSlug: action.slug };
    case "SET_LIBRARY_SLUG":
      return { ...state, librarySlug: action.slug };
    case "SET_VERSION":
      return {
        ...state,
        boundVersion:
          action.version === null
            ? null
            : {
                namespaceSlug: state.namespaceSlug,
                librarySlug: state.librarySlug,
                version: action.version,
              },
      };
    case "CLEAR_LIBRARY_DETAIL":
      // A failed (e.g. 404 private) Library detail refetch must not leave the
      // previous detail or version source_code mounted.
      return {
        ...state,
        data: {
          ...state.data,
          libraryDetail: null,
          versionSummaries: null,
          versionSource: null,
          versionCreated: null,
        },
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
