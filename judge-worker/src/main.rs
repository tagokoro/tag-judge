use std::{env, fs, process};

use judge_worker::{mock_sqs::MockSqs, models::JudgeJob, worker::process_next};

fn main() {
    let Some(queue_path) = env::args().nth(1) else {
        eprintln!("usage: judge-worker <mock-queue.json>");
        process::exit(2);
    };

    let queue_json = fs::read_to_string(&queue_path).unwrap_or_else(|error| {
        eprintln!("failed to read mock queue file: {}", error);
        process::exit(2);
    });

    let jobs: Vec<JudgeJob> = serde_json::from_str(&queue_json).unwrap_or_else(|error| {
        eprintln!("failed to parse mock queue JSON: {}", error);
        process::exit(2);
    });

    let mut queue = MockSqs::new(jobs);
    let mut results = Vec::new();

    while let Some(result) = process_next(&mut queue) {
        results.push(result);
    }

    let results_json = serde_json::to_string_pretty(&results).unwrap_or_else(|error| {
        eprintln!("failed to serialize worker results: {}", error);
        process::exit(1);
    });

    println!("{}", results_json);
}
