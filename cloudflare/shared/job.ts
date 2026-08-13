export type ExecutionReferenceKind = "submission" | "code_run";
export type ExecutionModeKind = "problem" | "stdin";
export type ExecutionLanguage = "python";
export type Verdict = "ac" | "wa" | "tle" | "mle" | "re" | "ce" | "ie";

export type ExecutionJob = {
  reference: {
    kind: ExecutionReferenceKind;
    id: string;
  };
  executionMode: ExecutionModeKind;
  submissionId?: string;
  codeRunId?: string;
  language: ExecutionLanguage;
  sourceCodeKey: string;
  testCasesKey?: string;
  stdin?: string;
  timeLimitMs: number;
  problemExecution?: {
    testCasesKey: string;
    timeLimitMs: number;
  };
  stdinExecution?: {
    stdin: string;
    timeLimitMs: number;
  };
};

export type TestCaseArtifact = {
  testCaseId: number;
  score: number;
  input: string;
  expectedOutput: string;
};

export type JudgeCaseResult = {
  testCaseId: number;
  caseIndex: number;
  verdict: Verdict;
  runtimeMs: number;
  stdout: string;
  stderr: string;
  message?: string;
};

export type JudgeRunnerResult = {
  verdict: Verdict;
  score: number;
  caseResults: JudgeCaseResult[];
  message?: string;
};
