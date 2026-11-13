// Client-side API helper. Every request goes to the SAME ORIGIN under /api/v1
// only. The Next.js rewrite (frontend/next.config.ts) proxies /api/:path* to
// the Go API server-side; the browser never learns API_SERVER_ORIGIN.
//
// All requests use `credentials: "same-origin"` so the HttpOnly session cookie
// is sent automatically. Development-only X-User-ID / X-User-Role headers are
// attached ONLY when the caller explicitly opts in (see devHeadersOf in
// lib/session.ts); a real cookie session always wins.

import type {
  CreateLibraryInput,
  CreateProfileInput,
  DevHeaders,
  Library,
  LibraryVersion,
  LibraryVersionSummary,
  Me,
  Category,
  Problem,
  PublicSubmission,
  SetSubmissionUrlInput,
  Submission,
  UpdateLibraryInput,
  UpdateCategoryInput,
  UserProfile,
} from "./types";

export const API_BASE = "/api/v1";
export const GOOGLE_LOGIN_PATH = "/api/v1/auth/google/login";
const LIST_LIMIT = 100;

export interface ApiRequestError extends Error {
  status: number;
  code: string;
}

function makeApiRequestError(status: number, code: string, message: string): ApiRequestError {
  return Object.assign(new Error(message), {
    name: "ApiRequestError",
    status,
    code,
  }) as ApiRequestError;
}

/**
 * The 404 a page answers with when a resource exists but the URL names another
 * owner: there is only one correct URL, so any other is "not found".
 */
export function notFoundError(): ApiRequestError {
  return makeApiRequestError(404, "NOT_FOUND", "not found");
}

export function isApiRequestError(value: unknown): value is ApiRequestError {
  return (
    typeof value === "object" &&
    value !== null &&
    (value as { name?: string }).name === "ApiRequestError"
  );
}

export function describeError(value: unknown): string {
  if (isApiRequestError(value)) {
    return `${value.code}: ${value.message} (HTTP ${value.status})`;
  }
  if (value instanceof Error) {
    return value.message;
  }
  return String(value);
}

export interface ApiInit extends Omit<RequestInit, "headers"> {
  headers?: Record<string, string>;
  json?: unknown;
}

/**
 * Low-level request. `fetchImpl` is injectable so this can be unit-tested with
 * Node's built-in test runner without a browser or a running API. `signal` (if
 * present in init) is forwarded to fetch so callers can abort in-flight work on
 * an actor change / unmount.
 */
export async function apiRequest<T>(
  path: string,
  init: ApiInit = {},
  devHeaders: DevHeaders | null = null,
  fetchImpl: typeof fetch = fetch,
): Promise<T> {
  const { json, headers = {}, body: rawBody, ...rest } = init;
  const finalHeaders: Record<string, string> = { ...headers };

  // Opt-in dev auth only. Never sent for an active cookie session.
  if (devHeaders) {
    finalHeaders["X-User-ID"] = devHeaders["X-User-ID"];
    finalHeaders["X-User-Role"] = devHeaders["X-User-Role"];
  }

  let body = rawBody;
  if (json !== undefined) {
    finalHeaders["Content-Type"] = "application/json";
    body = JSON.stringify(json);
  }

  const response = await fetchImpl(`${API_BASE}${path}`, {
    ...rest,
    // Always same-origin cookies; callers cannot override this.
    credentials: "same-origin",
    body,
    headers: finalHeaders,
  });

  // 204 (e.g. logout) carries no body.
  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  let parsed: unknown = undefined;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = undefined;
    }
  }

  if (!response.ok) {
    const error = (parsed as { error?: { code?: string; message?: string } } | undefined)?.error;
    throw makeApiRequestError(
      response.status,
      error?.code ?? "UNKNOWN_ERROR",
      error?.message ?? `HTTP ${response.status}`,
    );
  }

  return (parsed as { data?: T } | undefined)?.data as T;
}

/** Probe the cookie session with NO dev headers. 401 means "not logged in". */
export function probeCookieSession(fetchImpl: typeof fetch = fetch): Promise<Me> {
  return apiRequest<Me>("/me", {}, null, fetchImpl);
}

function segment(value: string): string {
  return encodeURIComponent(value.trim());
}

function list(path: string, limit = LIST_LIMIT): string {
  return `${path}?limit=${limit}`;
}

// Categories and libraries are addressed by their id alone.
export interface JudgeClient {
  getMe(): Promise<Me>;
  logout(): Promise<void>;
  /** Registers the caller's handle, once. */
  createProfile(input: CreateProfileInput): Promise<UserProfile>;
  getUser(handle: string): Promise<UserProfile>;
  /** Every category the caller owns, including ones others cannot see. */
  listMyCategories(): Promise<Category[]>;
  listCategories(handle: string): Promise<Category[]>;
  getCategory(categoryId: string): Promise<Category>;
  updateCategory(categoryId: string, patch: UpdateCategoryInput): Promise<Category>;
  listLibraries(categoryId: string): Promise<Library[]>;
  /** Files a new library under the caller's category named in the input. */
  createLibrary(input: CreateLibraryInput): Promise<Library>;
  getLibrary(libraryId: string): Promise<Library>;
  updateLibrary(libraryId: string, patch: UpdateLibraryInput): Promise<Library>;
  /** Newest first. `limit` defaults to the usual list size; pass 1 for just the latest. */
  listVersions(libraryId: string, limit?: number): Promise<LibraryVersionSummary[]>;
  getVersion(libraryId: string, version: number): Promise<LibraryVersion>;
  createVersion(libraryId: string, sourceCode: string): Promise<LibraryVersion>;
  // Site-wide lists.
  listPublicLibraries(): Promise<Library[]>;
  listProblems(): Promise<Problem[]>;
  listMyProblems(): Promise<Problem[]>;
  listRecentSubmissions(): Promise<PublicSubmission[]>;
  listMySubmissions(): Promise<Submission[]>;
  // The owner sets or clears a version's submission URL. Reads come with the
  // version.
  setSubmissionUrl(libraryId: string, version: number, input: SetSubmissionUrlInput): Promise<LibraryVersion>;
  clearSubmissionUrl(libraryId: string, version: number): Promise<void>;
  /** Return a client whose requests carry `signal` so they can be aborted. */
  withSignal(signal: AbortSignal | undefined): JudgeClient;
}

/**
 * Build a client bound to a `getDevHeaders` provider. The provider is called on
 * every request so the latest opt-in state is used (cookie session always wins).
 * `signal` is an optional default AbortSignal applied to every request.
 */
export function createJudgeClient(
  getDevHeaders: () => DevHeaders | null,
  fetchImpl?: typeof fetch,
  signal?: AbortSignal,
): JudgeClient {
  function call<T>(path: string, init: ApiInit = {}): Promise<T> {
    const merged: ApiInit = signal ? { ...init, signal } : init;
    return apiRequest<T>(path, merged, getDevHeaders(), fetchImpl ?? fetch);
  }

  const categoryBase = (categoryId: string) => `/categories/${segment(categoryId)}`;
  const libraryBase = (libraryId: string) => `/libraries/${segment(libraryId)}`;

  return {
    getMe: () => call<Me>("/me"),
    logout: () => call<void>("/auth/logout", { method: "POST" }),
    createProfile: (input) => call<UserProfile>("/me/profile", { method: "POST", json: input }),
    getUser: (handle) => call<UserProfile>(`/users/${segment(handle)}`),
    listMyCategories: () => call<Category[]>("/me/categories"),
    listCategories: (handle) => call<Category[]>(list(`/users/${segment(handle)}/categories`)),
    getCategory: (categoryId) => call<Category>(categoryBase(categoryId)),
    updateCategory: (categoryId, patch) =>
      call<Category>(categoryBase(categoryId), { method: "PATCH", json: patch }),
    listLibraries: (categoryId) => call<Library[]>(list(`${categoryBase(categoryId)}/libraries`)),
    createLibrary: (input) => call<Library>("/me/libraries", { method: "POST", json: input }),
    getLibrary: (libraryId) => call<Library>(libraryBase(libraryId)),
    updateLibrary: (libraryId, patch) => call<Library>(libraryBase(libraryId), { method: "PATCH", json: patch }),
    listVersions: (libraryId, limit) =>
      call<LibraryVersionSummary[]>(list(`${libraryBase(libraryId)}/versions`, limit)),
    getVersion: (libraryId, version) => call<LibraryVersion>(`${libraryBase(libraryId)}/versions/${version}`),
    createVersion: (libraryId, sourceCode) =>
      call<LibraryVersion>(`${libraryBase(libraryId)}/versions`, {
        method: "POST",
        json: { source_code: sourceCode },
      }),
    listPublicLibraries: () => call<Library[]>(list("/libraries")),
    listProblems: () => call<Problem[]>(list("/problems")),
    listMyProblems: () => call<Problem[]>(list("/me/problems")),
    listRecentSubmissions: () => call<PublicSubmission[]>(list("/submissions/recent")),
    listMySubmissions: () => call<Submission[]>(list("/submissions")),
    setSubmissionUrl: (libraryId, version, input) =>
      call<LibraryVersion>(`${libraryBase(libraryId)}/versions/${version}/submission-url`, { method: "PUT", json: input }),
    clearSubmissionUrl: (libraryId, version) =>
      call<void>(`${libraryBase(libraryId)}/versions/${version}/submission-url`, { method: "DELETE" }),
    withSignal: (nextSignal) => createJudgeClient(getDevHeaders, fetchImpl, nextSignal),
  };
}
