use std::process;

use judge_worker::{
    sqs::{SqsQueue, SqsQueueConfig},
    worker::process_job,
};

#[tokio::main]
async fn main() {
    if let Err(error) = run_once().await {
        eprintln!("{}", error);
        process::exit(1);
    }
}

async fn run_once() -> Result<(), String> {
    let queue = SqsQueue::connect(SqsQueueConfig::from_env()).await?;
    let Some(received) = queue.receive_job().await? else {
        println!("no judge job found");
        return Ok(());
    };

    let result = process_job(received.job);
    let result_json = serde_json::to_string_pretty(&result)
        .map_err(|error| format!("failed to serialize worker result: {}", error))?;

    println!("{}", result_json);
    queue.delete_message(&received.receipt_handle).await?;

    Ok(())
}
