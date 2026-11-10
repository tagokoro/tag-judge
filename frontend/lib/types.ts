// Types mirroring backend/docs/openapi.yaml (PR #45 URL-link contract) and the
// agreed design (docs/superpowers/specs/2026-11-10-verification-url-design.md).
// A submission link on a library version is ONLY a self-declared related-
// submission URL — not a verification/judge result. There is no adapter,
// Problem selection, Submission, Judge run, verdict, score, status, or case
// result in this flow. This module is type-only: it is imported with
// `import type` everywhere and never loaded at runtime, which also keeps the
// Node test runner happy.

export type Visibility = "public" | "private";
export type Role = "user" | "author" | "admin";

export interface Me {
  user_id: string;
  role: Role;
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

export interface LibraryVersion {
  id: string;
  version_number: number;
  source_code: string;
  content_sha256: string;
  byte_size: number;
  created_at: string;
}

export interface LibraryVersionSummary {
  id: string;
  version_number: number;
  content_sha256: string;
  byte_size: number;
  created_at: string;
}

// The existing, untouched judge Problems API (GET /problems). PR #47 does not
// change any judge/Submission behavior; it only replaces the library
// verification flow with self-declared URL links.
export interface Problem {
  id: number;
  title: string;
  statement: string;
  time_limit_ms: number;
  memory_limit_kb: number;
  visibility: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

/**
 * A self-declared "related submission" URL link (OpenAPI `SubmissionLink`).
 * The URL is never fetched, parsed for content, or judged: authenticity of the
 * submitter, verdict, or library version used is NOT guaranteed. Verdict,
 * score, status, adapter_source, case_results, judged_at, submission_id, and
 * problem_id are intentionally absent.
 */
export interface SubmissionLink {
  id: number;
  library_version_id: string;
  namespace_slug: string;
  library_slug: string;
  version_number: number;
  submission_url: string;
  created_by: string;
  created_at: string;
}

/** POST body for registering one link: a single absolute HTTP(S) URL. */
export interface CreateSubmissionLinkInput {
  submission_url: string;
}
