use std::{env, fs, process};

use judge_core::{judge, models::JudgeRequest};

fn main() {
    let Some(request_path) = env::args().nth(1) else {
        eprintln!("usage: judge-core <request.json>");
        process::exit(2);
    };

    let request_json = fs::read_to_string(&request_path).unwrap_or_else(|error| {
        eprintln!("failed to read request file: {}", error);
        process::exit(2);
    });

    let request: JudgeRequest = serde_json::from_str(&request_json).unwrap_or_else(|error| {
        eprintln!("failed to parse request JSON: {}", error);
        process::exit(2);
    });

    let result = judge::run(request);
    let result_json = serde_json::to_string_pretty(&result).unwrap_or_else(|error| {
        eprintln!("failed to serialize judge result: {}", error);
        process::exit(1);
    });

    println!("{}", result_json);
}
