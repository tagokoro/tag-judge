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
  CreateNamespaceInput,
  DevHeaders,
  Library,
  LibraryVersion,
  LibraryVersionSummary,
  Me,
  Namespace,
  Problem,
  SetSubmissionUrlInput,
  UpdateLibraryInput,
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

function list(path: string): string {
  return `${path}?limit=${LIST_LIMIT}`;
}

export interface JudgeClient {
  getMe(): Promise<Me>;
  logout(): Promise<void>;
  getMyNamespace(): Promise<Namespace>;
  getNamespace(namespaceSlug: string): Promise<Namespace>;
  createNamespace(input: CreateNamespaceInput): Promise<Namespace>;
  listLibraries(namespaceSlug: string): Promise<Library[]>;
  getLibrary(namespaceSlug: string, librarySlug: string): Promise<Library>;
  createLibrary(namespaceSlug: string, input: CreateLibraryInput): Promise<Library>;
  updateLibrary(
    namespaceSlug: string,
    librarySlug: string,
    patch: UpdateLibraryInput,
  ): Promise<Library>;
  listVersions(namespaceSlug: string, librarySlug: string): Promise<LibraryVersionSummary[]>;
  getVersion(namespaceSlug: string, librarySlug: string, version: number): Promise<LibraryVersion>;
  createVersion(namespaceSlug: string, librarySlug: string, sourceCode: string): Promise<LibraryVersion>;
  // The existing judge Problems API.
  listProblems(): Promise<Problem[]>;
  // The owner sets or clears a version's submission URL. Reads come with the
  // version.
  setSubmissionUrl(
    namespaceSlug: string,
    librarySlug: string,
    version: number,
    input: SetSubmissionUrlInput,
  ): Promise<LibraryVersion>;
  clearSubmissionUrl(namespaceSlug: string, librarySlug: string, version: number): Promise<void>;
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

  const libraryBase = (namespaceSlug: string, librarySlug: string) =>
    `/namespaces/${segment(namespaceSlug)}/libraries/${segment(librarySlug)}`;

  return {
    getMe: () => call<Me>("/me"),
    logout: () => call<void>("/auth/logout", { method: "POST" }),
    getMyNamespace: () => call<Namespace>("/me/namespace"),
    getNamespace: (namespaceSlug) => call<Namespace>(`/namespaces/${segment(namespaceSlug)}`),
    createNamespace: (input) =>
      call<Namespace>("/namespaces", { method: "POST", json: input }),
    listLibraries: (namespaceSlug) =>
      call<Library[]>(list(`/namespaces/${segment(namespaceSlug)}/libraries`)),
    getLibrary: (namespaceSlug, librarySlug) =>
      call<Library>(libraryBase(namespaceSlug, librarySlug)),
    createLibrary: (namespaceSlug, input) =>
      call<Library>(`/namespaces/${segment(namespaceSlug)}/libraries`, {
        method: "POST",
        json: input,
      }),
    updateLibrary: (namespaceSlug, librarySlug, patch) =>
      call<Library>(libraryBase(namespaceSlug, librarySlug), {
        method: "PATCH",
        json: patch,
      }),
    listVersions: (namespaceSlug, librarySlug) =>
      call<LibraryVersionSummary[]>(list(`${libraryBase(namespaceSlug, librarySlug)}/versions`)),
    getVersion: (namespaceSlug, librarySlug, version) =>
      call<LibraryVersion>(`${libraryBase(namespaceSlug, librarySlug)}/versions/${version}`),
    createVersion: (namespaceSlug, librarySlug, sourceCode) =>
      call<LibraryVersion>(`${libraryBase(namespaceSlug, librarySlug)}/versions`, {
        method: "POST",
        json: { source_code: sourceCode },
      }),
    listProblems: () => call<Problem[]>(list("/problems")),
    setSubmissionUrl: (namespaceSlug, librarySlug, version, input) =>
      call<LibraryVersion>(
        `${libraryBase(namespaceSlug, librarySlug)}/versions/${version}/submission-url`,
        { method: "PUT", json: input },
      ),
    clearSubmissionUrl: (namespaceSlug, librarySlug, version) =>
      call<void>(`${libraryBase(namespaceSlug, librarySlug)}/versions/${version}/submission-url`, {
        method: "DELETE",
      }),
    withSignal: (nextSignal) => createJudgeClient(getDevHeaders, fetchImpl, nextSignal),
  };
}
