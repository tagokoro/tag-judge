use std::process;

use judge_worker::{
    storage::{S3Storage, S3StorageConfig},
    sqs::{SqsQueue, SqsQueueConfig},
    worker::process_job,
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

    loop {
        let Some(received) = queue.receive_job().await? else {
            continue;
        };

        let result = process_job(received.job, &storage).await;
        let result_json = serde_json::to_string_pretty(&result)
            .map_err(|error| format!("failed to serialize worker result: {}", error))?;

        println!("{}", result_json);
        queue.delete_message(&received.receipt_handle).await?;
    }
}
