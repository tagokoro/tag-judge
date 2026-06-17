use judge_core::models::{JudgeResult, Language};
use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct JudgeJob {
    pub submission_id: String,
    pub language: Language,
    pub source_code_path: String,
    pub test_cases_path: String,
    pub time_limit_ms: u32,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkerResult {
    pub submission_id: String,
    pub result: JudgeResult,
}
