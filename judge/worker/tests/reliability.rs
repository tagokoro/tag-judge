use std::{
    future::Future,
    pin::Pin,
    sync::{
        Arc,
        atomic::{AtomicBool, AtomicUsize, Ordering},
    },
    time::Duration,
};

use judge_core::{models::Language, runner::direct::DirectRunner};
use judge_worker::{
    heartbeat::{HeartbeatFuture, VisibilityExtender, spawn_visibility_heartbeat},
    models::JudgeJob,
    orchestrator::{JobOutcome, handle_job_with_runner},
    result_sink::{ResultSink, StartOutcome},
    storage::ArtifactStore,
};

type StringFuture<'a, T> = Pin<Box<dyn Future<Output = Result<T, String>> + Send + 'a>>;

struct Artifacts;

impl ArtifactStore for Artifacts {
    fn read_source_code<'a>(&'a self, _key: &'a str) -> StringFuture<'a, String> {
        Box::pin(async { Ok("print(input())".to_string()) })
    }

    fn read_test_cases<'a>(&'a self, _key: &'a str) -> StringFuture<'a, String> {
        Box::pin(async {
            Ok(
                r#"[{"testCaseId":1,"score":1,"input":"ok\n","expectedOutput":"ok\n"}]"#
                    .to_string(),
            )
        })
    }
}

struct DatabaseUnavailable;

impl ResultSink for DatabaseUnavailable {
    fn mark_running<'a>(&'a self, _submission_id: &'a str) -> StringFuture<'a, StartOutcome> {
        Box::pin(async { Ok(StartOutcome::Started) })
    }

    fn complete<'a>(
        &'a self,
        _result: &'a judge_worker::models::WorkerResult,
    ) -> StringFuture<'a, ()> {
        Box::pin(async { Err("temporary database outage".to_string()) })
    }
}

struct TerminalSink;

impl ResultSink for TerminalSink {
    fn mark_running<'a>(&'a self, _submission_id: &'a str) -> StringFuture<'a, StartOutcome> {
        Box::pin(async { Ok(StartOutcome::AlreadyTerminal) })
    }

    fn complete<'a>(
        &'a self,
        _result: &'a judge_worker::models::WorkerResult,
    ) -> StringFuture<'a, ()> {
        Box::pin(async { panic!("terminal jobs must not be completed again") })
    }
}

fn job() -> JudgeJob {
    JudgeJob {
        submission_id: "1".to_string(),
        language: Language::Python,
        source_code_key: "source.py".to_string(),
        test_cases_key: "cases.json".to_string(),
        time_limit_ms: 1_000,
    }
}

#[tokio::test]
async fn database_failure_bubbles_out_so_the_sqs_message_is_not_acknowledged() {
    let error = match handle_job_with_runner(
        job(),
        &Artifacts,
        &DatabaseUnavailable,
        &DirectRunner::new(),
    )
    .await
    {
        Err(error) => error,
        Ok(_) => panic!("temporary DB failure must be retried"),
    };
    assert!(error.contains("temporary database outage"));
}

#[tokio::test]
async fn duplicate_terminal_job_is_acknowledgeable_without_rejudging() {
    let outcome = handle_job_with_runner(job(), &Artifacts, &TerminalSink, &DirectRunner::new())
        .await
        .expect("terminal duplicate is a successful no-op");
    assert!(matches!(outcome, JobOutcome::AlreadyTerminal));
}

#[derive(Clone)]
struct CountingVisibilityExtender {
    calls: Arc<AtomicUsize>,
}

impl VisibilityExtender for CountingVisibilityExtender {
    fn extend_visibility<'a>(
        &'a self,
        _receipt_handle: &'a str,
        _seconds: i32,
    ) -> HeartbeatFuture<'a> {
        let calls = Arc::clone(&self.calls);
        Box::pin(async move {
            calls.fetch_add(1, Ordering::SeqCst);
            Ok(())
        })
    }
}

#[tokio::test]
async fn visibility_heartbeat_extends_the_lease_until_processing_finishes() {
    let calls = Arc::new(AtomicUsize::new(0));
    let done = Arc::new(AtomicBool::new(false));
    let handle = spawn_visibility_heartbeat(
        CountingVisibilityExtender {
            calls: Arc::clone(&calls),
        },
        "receipt".to_string(),
        Arc::clone(&done),
        Duration::from_millis(5),
        30,
    );
    tokio::time::sleep(Duration::from_millis(20)).await;
    done.store(true, Ordering::Release);
    handle.await.expect("heartbeat task should stop cleanly");
    assert!(calls.load(Ordering::SeqCst) >= 1);
}
