"use client";

import { readDevAuth } from "@/lib/auth-store";
import type { Me, Problem, ProblemPayload, Submission, TestCase, TestCasePayload } from "@/lib/types";

type Wrapped<T> = { data: T };
type ErrorBody = { error?: { code?: string; message?: string } };

const apiBase = process.env.NEXT_PUBLIC_API_BASE_PATH ?? "/api/backend";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const auth = readDevAuth();
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (auth) {
    headers.set("X-User-ID", auth.userId);
    headers.set("X-User-Role", auth.role);
  }

  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    headers,
    credentials: "include",
    cache: "no-store"
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const body = (await response.json().catch(() => ({}))) as Wrapped<T> & ErrorBody;
  if (!response.ok) {
    throw new ApiError(
      response.status,
      body.error?.code ?? "REQUEST_FAILED",
      body.error?.message ?? `Request failed with status ${response.status}`
    );
  }
  return body.data;
}

export const api = {
  me: () => apiRequest<Me>("/me"),
  logout: () => apiRequest<void>("/auth/logout", { method: "POST" }),
  listProblems: () => apiRequest<Problem[]>("/problems?limit=100"),
  getProblem: (id: number) => apiRequest<Problem>(`/problems/${id}`),
  createProblem: (payload: ProblemPayload) =>
    apiRequest<Problem>("/problems", { method: "POST", body: JSON.stringify(payload) }),
  updateProblem: (id: number, payload: ProblemPayload) =>
    apiRequest<Problem>(`/problems/${id}`, { method: "PUT", body: JSON.stringify(payload) }),
  deleteProblem: (id: number) => apiRequest<void>(`/problems/${id}`, { method: "DELETE" }),
  listTestCases: (problemId: number) => apiRequest<TestCase[]>(`/problems/${problemId}/test-cases`),
  createTestCase: (problemId: number, payload: TestCasePayload) =>
    apiRequest<TestCase>(`/problems/${problemId}/test-cases`, { method: "POST", body: JSON.stringify(payload) }),
  updateTestCase: (problemId: number, testCaseId: number, payload: TestCasePayload) =>
    apiRequest<TestCase>(`/problems/${problemId}/test-cases/${testCaseId}`, {
      method: "PUT",
      body: JSON.stringify(payload)
    }),
  deleteTestCase: (problemId: number, testCaseId: number) =>
    apiRequest<void>(`/problems/${problemId}/test-cases/${testCaseId}`, { method: "DELETE" }),
  createSubmission: (problemId: number, sourceCode: string) =>
    apiRequest<Submission>(`/problems/${problemId}/submissions`, {
      method: "POST",
      body: JSON.stringify({ language: "python", source_code: sourceCode })
    }),
  listSubmissions: (problemId: number) => apiRequest<Submission[]>(`/problems/${problemId}/submissions`),
  getSubmission: (submissionId: number) => apiRequest<Submission>(`/submissions/${submissionId}`)
};
