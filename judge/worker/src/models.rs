use judge_core::models::{Language, Verdict};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct JudgeJob {
    pub submission_id: String,
    pub language: Language,
    pub source_code_key: String,
    pub test_cases_key: String,
    pub time_limit_ms: u32,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TestCaseArtifact {
    pub test_case_id: i64,
    pub score: i32,
    pub input: String,
    pub expected_output: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkerResult {
    pub submission_id: String,
    pub test_cases_key: String,
    pub score: i32,
    pub result: WorkerJudgeResult,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkerJudgeResult {
    pub verdict: Verdict,
    pub case_results: Vec<WorkerCaseResult>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkerCaseResult {
    pub test_case_id: i64,
    pub case_index: usize,
    pub verdict: Verdict,
    pub runtime_ms: u128,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
}
