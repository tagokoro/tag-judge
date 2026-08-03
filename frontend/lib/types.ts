export type Role = "user" | "author" | "admin";
export type Visibility = "public" | "private" | "group";
export type SubmissionStatus = "pending" | "running" | "ac" | "wa" | "tle" | "mle" | "re" | "ce" | "ie";

export type Me = {
  user_id: string;
  role: Role;
};

export type Problem = {
  id: number;
  title: string;
  statement: string;
  time_limit_ms: number;
  memory_limit_kb: number;
  visibility: Visibility;
  created_by: string;
  created_at: string;
  updated_at: string;
};

export type ProblemPayload = {
  title: string;
  statement: string;
  time_limit_ms: number;
  memory_limit_kb: number;
  visibility: Visibility;
};

export type TestCase = {
  id: number;
  problem_id: number;
  input: string;
  expected_output: string;
  is_sample: boolean;
  score: number;
  created_at: string;
};

export type TestCasePayload = {
  input: string;
  expected_output: string;
  is_sample: boolean;
  score: number;
};

export type SubmissionResult = {
  id: number;
  submission_id: number;
  test_case_id: number;
  status: SubmissionStatus;
  execution_time_ms: number | null;
  memory_kb: number | null;
  stdout: string;
  stderr: string;
  created_at: string;
};

export type Submission = {
  id: number;
  user_id: string;
  problem_id: number;
  language: "python";
  source_code: string;
  status: SubmissionStatus;
  score: number;
  created_at: string;
  judged_at: string | null;
  results: SubmissionResult[];
};

export type DevAuth = {
  userId: string;
  role: Role;
};
