import { getSandbox, type Sandbox } from "@cloudflare/sandbox";
import type { ExecutionJob, JudgeRunnerResult } from "../../shared/job";

export { Sandbox } from "@cloudflare/sandbox";

type Env = {
  DB: D1Database;
  JUDGE_STORAGE: R2Bucket;
  Sandbox: DurableObjectNamespace<Sandbox>;
};

export default {
  async queue(batch: MessageBatch<ExecutionJob>, env: Env): Promise<void> {
    for (const message of batch.messages) {
      await processJob(message.body, env);
      message.ack();
    }
  },
};

async function processJob(job: ExecutionJob, env: Env): Promise<void> {
  if (job.reference.kind !== "submission" || job.executionMode !== "problem") {
    throw new Error(`unsupported execution job: ${job.reference.kind}/${job.executionMode}`);
  }
  if (!job.submissionId || !job.testCasesKey) {
    throw new Error("submissionId and testCasesKey are required");
  }

  const submission = await env.DB.prepare("SELECT status FROM submissions WHERE id = ?")
    .bind(job.submissionId)
    .first<{ status: string }>();
  if (!submission) {
    throw new Error(`submission ${job.submissionId} not found`);
  }
  if (isTerminalStatus(submission.status)) {
    return;
  }

  await env.DB.prepare("UPDATE submissions SET status = 'running' WHERE id = ?")
    .bind(job.submissionId)
    .run();

  const source = await readObjectText(env.JUDGE_STORAGE, job.sourceCodeKey);
  const testCases = await readObjectText(env.JUDGE_STORAGE, job.testCasesKey);
  const result = await runJudge(env, job, source, testCases);
  await persistResult(env.DB, job, result);
}

async function runJudge(
  env: Env,
  job: ExecutionJob,
  sourceCode: string,
  testCases: string,
): Promise<JudgeRunnerResult> {
  const sandbox = getSandbox(env.Sandbox, `submission-${job.submissionId}`);
  await sandbox.writeFile("/workspace/main.py", sourceCode);
  await sandbox.writeFile("/workspace/testcases.json", testCases);

  const command = [
    "judge-runner",
    "--language",
    job.language,
    "--source",
    "/workspace/main.py",
    "--testcases",
    "/workspace/testcases.json",
    "--time-limit-ms",
    String(job.timeLimitMs),
  ].join(" ");
  const output = await sandbox.exec(command);
  if (!output.success) {
    return {
      verdict: "ie",
      score: 0,
      caseResults: [],
      message: output.stderr || `judge-runner exited with ${output.exitCode}`,
    };
  }
  return JSON.parse(output.stdout) as JudgeRunnerResult;
}

async function persistResult(db: D1Database, job: ExecutionJob, result: JudgeRunnerResult): Promise<void> {
  const submissionId = Number(job.submissionId);
  await db.batch([
    db.prepare("DELETE FROM submission_results WHERE submission_id = ?").bind(submissionId),
    ...result.caseResults.map((caseResult) =>
      db
        .prepare(
          `
            INSERT INTO submission_results (
              submission_id,
              test_case_id,
              status,
              execution_time_ms,
              memory_kb,
              stdout,
              stderr
            )
            VALUES (?, ?, ?, ?, NULL, ?, ?)
          `,
        )
        .bind(
          submissionId,
          caseResult.testCaseId,
          caseResult.verdict,
          caseResult.runtimeMs,
          caseResult.stdout,
          caseResult.stderr,
        ),
    ),
    db
      .prepare("UPDATE submissions SET status = ?, score = ?, judged_at = CURRENT_TIMESTAMP WHERE id = ?")
      .bind(result.verdict, result.score, submissionId),
  ]);
}

async function readObjectText(bucket: R2Bucket, key: string): Promise<string> {
  const object = await bucket.get(key);
  if (!object) {
    throw new Error(`artifact not found: ${key}`);
  }
  return object.text();
}

function isTerminalStatus(status: string): boolean {
  return ["ac", "wa", "tle", "mle", "re", "ce", "ie"].includes(status);
}
