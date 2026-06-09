use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct JudgeRequest {
    pub language: Language,
    pub source_code: String,
    pub time_limit_ms: u32,
    pub test_cases: Vec<TestCase>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TestCase {
    pub input: String,
    pub expected_output: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct JudgeResult {
    pub verdict: Verdict,
    pub case_results: Vec<CaseResult>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CaseResult {
    pub case_index: usize,
    pub verdict: Verdict,
    pub runtime_ms: u128,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Language {
    Python,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
pub enum Verdict {
    #[serde(rename = "AC")]
    AC,
    #[serde(rename = "WA")]
    WA,
    #[serde(rename = "TLE")]
    TLE,
    #[serde(rename = "RE")]
    RE,
    #[serde(rename = "CE")]
    CE,
    #[serde(rename = "IE")]
    IE,
}
