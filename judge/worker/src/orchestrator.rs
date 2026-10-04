use crate::{
    models::{JudgeJob, WorkerResult},
    result_sink::{ResultSink, StartOutcome},
    storage::ArtifactStore,
    worker::{process_job_reliably, process_job_with_runner_reliably},
};
use judge_core::runner::Runner;

pub enum JobOutcome {
    Completed(WorkerResult),
    AlreadyTerminal,
}

// Queue deletion is deliberately outside this function: it is legal only after
// complete() has committed. Any storage/database error bubbles out, retaining
// the SQS message for retry.
pub async fn handle_job(
    job: JudgeJob,
    store: &dyn ArtifactStore,
    sink: &dyn ResultSink,
) -> Result<JobOutcome, String> {
    match sink.mark_running(&job.submission_id).await? {
        StartOutcome::AlreadyTerminal => return Ok(JobOutcome::AlreadyTerminal),
        StartOutcome::Started => {}
    }
    let result = process_job_reliably(job, store)
        .await
        .map_err(|error| error.to_string())?;
    sink.complete(&result).await?;
    Ok(JobOutcome::Completed(result))
}

pub async fn handle_job_with_runner(
    job: JudgeJob,
    store: &dyn ArtifactStore,
    sink: &dyn ResultSink,
    runner: &dyn Runner,
) -> Result<JobOutcome, String> {
    match sink.mark_running(&job.submission_id).await? {
        StartOutcome::AlreadyTerminal => return Ok(JobOutcome::AlreadyTerminal),
        StartOutcome::Started => {}
    }
    let result = process_job_with_runner_reliably(job, store, runner)
        .await
        .map_err(|error| error.to_string())?;
    sink.complete(&result).await?;
    Ok(JobOutcome::Completed(result))
}
