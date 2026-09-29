use std::{
    process,
    sync::{
        Arc,
        atomic::{AtomicBool, Ordering},
    },
    time::Duration,
};

use judge_worker::{
    heartbeat::spawn_visibility_heartbeat,
    orchestrator::{JobOutcome, handle_job},
    result_sink::{PostgresResultSink, PostgresResultSinkConfig},
    sqs::{SqsQueue, SqsQueueConfig},
    storage::{S3Storage, S3StorageConfig},
};

#[tokio::main]
async fn main() {
    if let Err(error) = run_forever().await {
        eprintln!("{}", error);
        process::exit(1);
    }
}

async fn run_forever() -> Result<(), String> {
    let queue = SqsQueue::connect(SqsQueueConfig::from_env()).await?;
    let storage = S3Storage::connect(S3StorageConfig::from_env()).await?;
    let result_sink = PostgresResultSink::connect(PostgresResultSinkConfig::from_env()?).await?;

    loop {
        let Some(received) = queue.receive_job().await? else {
            continue;
        };
        // Keep the lease alive while a judge run exceeds the queue's default
        // visibility timeout. A failed extension makes the job eligible for an
        // at-least-once redelivery; DB-side idempotency handles that safely.
        let completed = Arc::new(AtomicBool::new(false));
        let heartbeat_done = Arc::clone(&completed);
        let heartbeat_queue = queue.clone();
        let heartbeat_receipt = received.receipt_handle.clone();
        let heartbeat = spawn_visibility_heartbeat(
            heartbeat_queue,
            heartbeat_receipt,
            heartbeat_done,
            Duration::from_secs(10),
            SqsQueue::default_visibility_timeout_seconds(),
        );

        let outcome = handle_job(received.job, &storage, &result_sink).await;
        completed.store(true, Ordering::Release);
        let _ = heartbeat.await;
        match outcome? {
            JobOutcome::Completed(result) => println!(
                "{}",
                serde_json::to_string_pretty(&result)
                    .map_err(|e| format!("failed to serialize worker result: {e}"))?
            ),
            JobOutcome::AlreadyTerminal => {
                println!("submission result already terminal; skipping duplicate job")
            }
        }
        queue.delete_message(&received.receipt_handle).await?;
    }
}
