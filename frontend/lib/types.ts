// Types mirroring backend/docs/openapi.yaml. This module is type-only: it is imported with
// `import type` everywhere and never loaded at runtime, which also keeps the
// Node test runner happy.

export type Visibility = "public" | "private";
export type Role = "user" | "author" | "admin";

export interface Me {
  user_id: string;
  role: Role;
  /** The user's only name. Null until they register at /signup. */
  handle: string | null;
}

/** A user's public profile (GET /users/{handle}). */
export interface UserProfile {
  handle: string;
}

export interface CreateProfileInput {
  handle: string;
}

export interface DevHeaders {
  "X-User-ID": string;
  "X-User-Role": string;
}

export interface Namespace {
  id: string;
  slug: string;
  display_name: string;
  owner_user_id: string;
  created_at: string;
  updated_at: string;
}

export interface CreateNamespaceInput {
  slug: string;
  display_name: string;
}

export interface Library {
  id: string;
  namespace_slug: string;
  slug: string;
  name: string;
  description: string;
  visibility: Visibility;
  created_at: string;
  updated_at: string;
}

export interface CreateLibraryInput {
  slug: string;
  name: string;
  description: string;
  visibility: Visibility;
}

export interface UpdateLibraryInput {
  name?: string;
  description?: string;
  visibility?: Visibility;
}

/**
 * submission_url is the owner's self-declared "related submission" URL (null
 * when unset). It is never fetched, parsed for content, or judged: authenticity
 * of the submitter, verdict, or library version used is NOT guaranteed.
 */
export interface LibraryVersion {
  id: string;
  version_number: number;
  source_code: string;
  content_sha256: string;
  byte_size: number;
  created_at: string;
  submission_url: string | null;
  submission_url_updated_at: string | null;
}

export interface LibraryVersionSummary {
  id: string;
  version_number: number;
  content_sha256: string;
  byte_size: number;
  created_at: string;
  submission_url: string | null;
  submission_url_updated_at: string | null;
}

// The existing judge Problems API (GET /problems).
export interface Problem {
  id: number;
  title: string;
  statement: string;
  time_limit_ms: number;
  memory_limit_kb: number;
  visibility: string;
  /** The creator's handle, or null when they have none. */
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export type SubmissionStatus = "pending" | "running" | "ac" | "wa" | "tle" | "mle" | "re" | "ce" | "ie";

/** The signed-in user's own submission (GET /submissions). */
export interface Submission {
  id: number;
  user_id: string;
  problem_id: number;
  language: string;
  status: SubmissionStatus;
  score: number;
  created_at: string;
  judged_at: string | null;
}

/** A submission to a public problem, as anyone may see it (no source, no user id). */
export interface PublicSubmission {
  id: number;
  problem_id: number;
  problem_title: string;
  language: string;
  status: SubmissionStatus;
  score: number;
  created_at: string;
  judged_at: string | null;
  /** The submitter's handle, or null when they have none. */
  submitter: string | null;
}

/** PUT body for setting a version's submission URL: one absolute HTTP(S) URL. */
export interface SetSubmissionUrlInput {
  submission_url: string;
}
