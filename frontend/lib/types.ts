// Types mirroring backend/docs/openapi.yaml (reviewed PR2 contract) and the
// agreed MVP design (docs/superpowers/specs/2026-11-08-library-sharing-mvp-design.md).
// This module is type-only: it is imported with `import type` everywhere and
// never loaded at runtime, which also keeps the Node test runner happy.

export type Visibility = "public" | "private";
export type Role = "user" | "author" | "admin";
export type JudgeStatus =
  | "pending"
  | "running"
  | "ac"
  | "wa"
  | "tle"
  | "mle"
  | "re"
  | "ce"
  | "ie";

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

export interface CaseResult {
  test_case_id: number;
  status: JudgeStatus;
  execution_time_ms: number | null;
  memory_kb: number | null;
}

export interface VerificationSummary {
  id: number;
  library_version_id: string;
  namespace_slug: string;
  library_slug: string;
  version_number: number;
  problem_id: number;
  submission_id: number;
  status: JudgeStatus;
  score: number;
  adapter_sha256: string;
  created_at: string;
  judged_at: string | null;
}

export interface VerificationDetail extends VerificationSummary {
  adapter_source: string;
  case_results: CaseResult[];
}

export interface CreateVerificationInput {
  problem_id: number;
  adapter_source: string;
}
