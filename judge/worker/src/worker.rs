use judge_core::{
    models::{JudgeRequest, JudgeResult, TestCase, Verdict},
    runner::{Runner, default_runner},
};

use crate::{
    mock_sqs::MockSqs,
    models::{JudgeJob, TestCaseArtifact, WorkerCaseResult, WorkerJudgeResult, WorkerResult},
    storage::ArtifactStore,
};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RetryableInfrastructureError(pub String);

impl std::fmt::Display for RetryableInfrastructureError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for RetryableInfrastructureError {}

pub async fn process_next(queue: &mut MockSqs, store: &dyn ArtifactStore) -> Option<WorkerResult> {
    match queue.receive_message() {
        Some(job) => Some(process_job(job, store).await),
        None => None,
    }
}

pub async fn process_next_with_runner(
    queue: &mut MockSqs,
    store: &dyn ArtifactStore,
    runner: &dyn Runner,
) -> Option<WorkerResult> {
    match queue.receive_message() {
        Some(job) => Some(process_job_with_runner(job, store, runner).await),
        None => None,
    }
}

pub async fn process_job(job: JudgeJob, store: &dyn ArtifactStore) -> WorkerResult {
    let fallback = job.clone();
    process_job_reliably(job, store)
        .await
        .unwrap_or_else(|error| internal_error(&fallback, error.to_string()))
}

// Infrastructure reads are deliberately outside the terminal-result path:
// persisting IE for a transient S3 failure would let the caller delete a job
// that has never actually been judged.
pub async fn process_job_reliably(
    job: JudgeJob,
    store: &dyn ArtifactStore,
) -> Result<WorkerResult, RetryableInfrastructureError> {
    match default_runner() {
        Ok(runner) => process_job_with_runner_reliably(job, store, runner.as_ref()).await,
        Err(message) => Ok(internal_error(&job, message)),
    }
}

pub async fn process_job_with_runner(
    job: JudgeJob,
    store: &dyn ArtifactStore,
    runner: &dyn Runner,
) -> WorkerResult {
    let fallback = job.clone();
    process_job_with_runner_reliably(job, store, runner)
        .await
        .unwrap_or_else(|error| internal_error(&fallback, error.to_string()))
}

pub async fn process_job_with_runner_reliably(
    job: JudgeJob,
    store: &dyn ArtifactStore,
    runner: &dyn Runner,
) -> Result<WorkerResult, RetryableInfrastructureError> {
    let source_code = store
        .read_source_code(&job.source_code_key)
        .await
        .map_err(RetryableInfrastructureError)?;
    let test_cases_json = store
        .read_test_cases(&job.test_cases_key)
        .await
        .map_err(RetryableInfrastructureError)?;
    match build_request_from_artifacts(&job, source_code, test_cases_json) {
        Ok((request, artifacts)) => build_worker_result(
            &job,
            artifacts,
            judge_core::judge::run_with_runner(request, runner),
        )
        .map_err(RetryableInfrastructureError),
        // A malformed immutable artifact is a deterministic judge-input
        // failure, so it is stored as IE and may be safely acknowledged.
        Err(message) => Ok(internal_error(&job, message)),
    }
}

fn build_request_from_artifacts(
    job: &JudgeJob,
    source_code: String,
    test_cases_json: String,
) -> Result<(JudgeRequest, Vec<TestCaseArtifact>), String> {
    let artifacts: Vec<TestCaseArtifact> = serde_json::from_str(&test_cases_json)
        .map_err(|error| format!("failed to parse test cases JSON: {}", error))?;
    if artifacts.is_empty() {
        return Err("test cases artifact must contain at least one case".to_string());
    }
    let mut test_cases = Vec::with_capacity(artifacts.len());
    for artifact in &artifacts {
        if artifact.test_case_id <= 0 || artifact.score <= 0 {
            return Err("test case artifact id and score must be positive".to_string());
        }
        test_cases.push(TestCase {
            input: artifact.input.clone(),
            expected_output: artifact.expected_output.clone(),
        });
    }
    Ok((
        JudgeRequest {
            language: job.language,
            source_code,
            time_limit_ms: job.time_limit_ms,
            test_cases,
        },
        artifacts,
    ))
}

fn build_worker_result(
    job: &JudgeJob,
    artifacts: Vec<TestCaseArtifact>,
    result: JudgeResult,
) -> Result<WorkerResult, String> {
    let score = if result.verdict == Verdict::AC {
        artifacts.iter().try_fold(0_i32, |total, artifact| {
            total
                .checked_add(artifact.score)
                .ok_or_else(|| "test case artifact total score is too large".to_string())
        })?
    } else {
        0
    };
    let mut case_results = Vec::with_capacity(result.case_results.len());
    for case_result in result.case_results {
        let artifact = artifacts.get(case_result.case_index).ok_or_else(|| {
            format!(
                "judge result referenced unknown case index {}",
                case_result.case_index
            )
        })?;
        case_results.push(WorkerCaseResult {
            test_case_id: artifact.test_case_id,
            case_index: case_result.case_index,
            verdict: case_result.verdict,
            runtime_ms: case_result.runtime_ms,
            stdout: case_result.stdout,
            stderr: case_result.stderr,
            message: case_result.message,
        });
    }
    Ok(WorkerResult {
        submission_id: job.submission_id.clone(),
        test_cases_key: job.test_cases_key.clone(),
        score,
        result: WorkerJudgeResult {
            verdict: result.verdict,
            case_results,
            message: result.message,
        },
    })
}

fn internal_error(job: &JudgeJob, message: String) -> WorkerResult {
    WorkerResult {
        submission_id: job.submission_id.clone(),
        test_cases_key: job.test_cases_key.clone(),
        score: 0,
        result: WorkerJudgeResult {
            verdict: Verdict::IE,
            case_results: vec![],
            message: Some(message),
        },
    }
}
