use std::fs;

use judge_core::{
    models::{JudgeRequest, TestCase, Verdict},
    runner::{Runner, default_runner},
};

use crate::{
    mock_sqs::MockSqs,
    models::{JudgeJob, WorkerResult},
};

pub fn process_next(queue: &mut MockSqs) -> Option<WorkerResult> {
    queue.receive_message().map(process_job)
}

pub fn process_next_with_runner(queue: &mut MockSqs, runner: &dyn Runner) -> Option<WorkerResult> {
    queue
        .receive_message()
        .map(|job| process_job_with_runner(job, runner))
}

pub fn process_job(job: JudgeJob) -> WorkerResult {
    match default_runner() {
        Ok(runner) => process_job_with_runner(job, runner.as_ref()),
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

pub fn process_job_with_runner(job: JudgeJob, runner: &dyn Runner) -> WorkerResult {
    let submission_id = job.submission_id.clone();
    let result = match build_request(job) {
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

fn build_request(job: JudgeJob) -> Result<JudgeRequest, String> {
    let source_code = fs::read_to_string(&job.source_code_path)
        .map_err(|error| format!("failed to read source code: {}", error))?;

    let test_cases_json = fs::read_to_string(&job.test_cases_path)
        .map_err(|error| format!("failed to read test cases: {}", error))?;

    let test_cases: Vec<TestCase> = serde_json::from_str(&test_cases_json)
        .map_err(|error| format!("failed to parse test cases JSON: {}", error))?;

    Ok(JudgeRequest {
        language: job.language,
        source_code,
        time_limit_ms: job.time_limit_ms,
        test_cases,
    })
}
