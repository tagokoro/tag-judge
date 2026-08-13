import { errorResponse, jsonResponse } from "../../shared/http";
import type { ExecutionJob, ExecutionLanguage, TestCaseArtifact } from "../../shared/job";

type Env = {
  DB: D1Database;
  JUDGE_STORAGE: R2Bucket;
  JUDGE_QUEUE: Queue<ExecutionJob>;
};

type CreateSubmissionBody = {
  language?: unknown;
  source_code?: unknown;
};

type TestCaseRow = {
  id: number;
  input: string;
  expected_output: string;
  score: number;
};

const supportedLanguages = new Set<ExecutionLanguage>(["python"]);

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    if (request.method === "GET" && url.pathname === "/healthz") {
      return jsonResponse({ status: "ok" });
    }

    const submissionMatch = url.pathname.match(/^\/api\/v1\/problems\/(\d+)\/submissions\/?$/);
    if (request.method === "POST" && submissionMatch) {
      return createSubmission(request, env, Number(submissionMatch[1]));
    }

    return errorResponse(404, "NOT_FOUND", "route not found");
  },
};

async function createSubmission(request: Request, env: Env, problemId: number): Promise<Response> {
  const userId = request.headers.get("x-user-id");
  if (!userId) {
    return errorResponse(401, "UNAUTHORIZED", "x-user-id header is required until session auth is ported");
  }

  const body = (await request.json().catch(() => null)) as CreateSubmissionBody | null;
  if (!body || typeof body.source_code !== "string" || typeof body.language !== "string") {
    return errorResponse(400, "INVALID_REQUEST", "language and source_code are required");
  }
  if (!supportedLanguages.has(body.language as ExecutionLanguage)) {
    return errorResponse(400, "UNSUPPORTED_LANGUAGE", "language must be python");
  }

  const problem = await env.DB.prepare(
    "SELECT id, time_limit_ms FROM problems WHERE id = ? AND visibility = 'public'",
  )
    .bind(problemId)
    .first<{ id: number; time_limit_ms: number }>();
  if (!problem) {
    return errorResponse(404, "PROBLEM_NOT_FOUND", "problem not found");
  }

  const testCases = await env.DB.prepare(
    "SELECT id, input, expected_output, score FROM test_cases WHERE problem_id = ? ORDER BY id",
  )
    .bind(problemId)
    .all<TestCaseRow>();
  if (!testCases.results.length) {
    return errorResponse(400, "TEST_CASE_NOT_FOUND", "problem has no test cases");
  }

  const artifact: TestCaseArtifact[] = testCases.results.map((testCase) => ({
    testCaseId: testCase.id,
    score: testCase.score,
    input: testCase.input,
    expectedOutput: testCase.expected_output,
  }));
  const testCasesKey = await putTestCases(env.JUDGE_STORAGE, problemId, artifact);

  const sourceCodeKey = sourceKey(userId, problemId, body.language);
  await env.JUDGE_STORAGE.put(sourceCodeKey, body.source_code, {
    httpMetadata: {
      contentType: sourceContentType(body.language),
    },
  });

  const submission = await env.DB.prepare(
    `
      INSERT INTO submissions (user_id, problem_id, language, source_code_key, test_cases_key, status, score)
      VALUES (?, ?, ?, ?, ?, 'pending', 0)
      RETURNING id
    `,
  )
    .bind(userId, problemId, body.language, sourceCodeKey, testCasesKey)
    .first<{ id: number }>();
  if (!submission) {
    return errorResponse(500, "SUBMISSION_CREATE_FAILED", "failed to create submission");
  }

  const job: ExecutionJob = {
    reference: {
      kind: "submission",
      id: String(submission.id),
    },
    executionMode: "problem",
    submissionId: String(submission.id),
    language: body.language as ExecutionLanguage,
    sourceCodeKey,
    testCasesKey,
    timeLimitMs: problem.time_limit_ms,
    problemExecution: {
      testCasesKey,
      timeLimitMs: problem.time_limit_ms,
    },
  };
  await env.JUDGE_QUEUE.send(job);

  return jsonResponse(
    {
      data: {
        id: submission.id,
        user_id: userId,
        problem_id: problemId,
        language: body.language,
        status: "pending",
        score: 0,
        source_code_key: sourceCodeKey,
        test_cases_key: testCasesKey,
      },
    },
    { status: 201 },
  );
}

async function putTestCases(
  bucket: R2Bucket,
  problemId: number,
  artifact: TestCaseArtifact[],
): Promise<string> {
  const json = JSON.stringify(artifact);
  const digest = await sha256Hex(json);
  const key = `problems/${problemId}/testcases/${digest}.json`;
  await bucket.put(key, json, {
    httpMetadata: {
      contentType: "application/json; charset=utf-8",
    },
  });
  return key;
}

async function sha256Hex(value: string): Promise<string> {
  const bytes = new TextEncoder().encode(value);
  const hash = await crypto.subtle.digest("SHA-256", bytes);
  return [...new Uint8Array(hash)].map((byte) => byte.toString(16).padStart(2, "0")).join("");
}

function sourceKey(userId: string, problemId: number, language: string): string {
  const extension = language === "python" ? "py" : "txt";
  return `submissions/${userId}/problems/${problemId}/${crypto.randomUUID()}/main.${extension}`;
}

function sourceContentType(language: string): string {
  if (language === "python") {
    return "text/x-python; charset=utf-8";
  }
  return "text/plain; charset=utf-8";
}
