use judge_core::{
    models::{JudgeRequest, TestCase, Verdict},
    runner::{Runner, default_runner},
};

use crate::{
    mock_sqs::MockSqs,
    models::{JudgeJob, WorkerResult},
    storage::ArtifactStore,
};

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
    match default_runner() {
        Ok(runner) => process_job_with_runner(job, store, runner.as_ref()).await,
        Err(message) => WorkerResult {
            submission_id: job.submission_id,
            result: judge_core::models::JudgeResult {
                verdict: Verdict::IE,
                case_results: vec![],
                message: Some(message),
            },
        },
    }
}

pub async fn process_job_with_runner(
    job: JudgeJob,
    store: &dyn ArtifactStore,
    runner: &dyn Runner,
) -> WorkerResult {
    let submission_id = job.submission_id.clone();
    let result = match build_request(&job, store).await {
        Ok(request) => judge_core::judge::run_with_runner(request, runner),
        Err(message) => judge_core::models::JudgeResult {
            verdict: Verdict::IE,
            case_results: vec![],
            message: Some(message),
        },
    };

    WorkerResult {
        submission_id,
        result,
    }
}

async fn build_request(job: &JudgeJob, store: &dyn ArtifactStore) -> Result<JudgeRequest, String> {
    let source_code = store.read_source_code(&job.source_code_key).await?;
    let test_cases_json = store.read_test_cases(&job.test_cases_key).await?;
    build_request_from_artifacts(job, source_code, test_cases_json)
}

fn build_request_from_artifacts(
    job: &JudgeJob,
    source_code: String,
    test_cases_json: String,
) -> Result<JudgeRequest, String> {
    let test_cases: Vec<TestCase> = serde_json::from_str(&test_cases_json)
        .map_err(|error| format!("failed to parse test cases JSON: {}", error))?;

    Ok(JudgeRequest {
        language: job.language,
        source_code,
        time_limit_ms: job.time_limit_ms,
        test_cases,
    })
}
