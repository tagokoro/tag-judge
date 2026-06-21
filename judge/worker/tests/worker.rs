use std::fs;
use std::path::{Path, PathBuf};

use judge_core::{
    models::{Language, Verdict},
    runner::direct::DirectRunner,
};
use judge_worker::{
    mock_sqs::MockSqs,
    models::JudgeJob,
    worker::{process_job_with_runner, process_next_with_runner},
};

fn write_file(dir: &Path, relative_path: &str, content: &str) -> PathBuf {
    let path = dir.join(relative_path);
    fs::create_dir_all(path.parent().expect("path should have parent")).unwrap();
    fs::write(&path, content).unwrap();
    path
}

fn job(
    submission_id: &str,
    source_code_path: PathBuf,
    test_cases_path: PathBuf,
    time_limit_ms: u32,
) -> JudgeJob {
    JudgeJob {
        submission_id: submission_id.to_string(),
        language: Language::Python,
        source_code_path: source_code_path.to_string_lossy().to_string(),
        test_cases_path: test_cases_path.to_string_lossy().to_string(),
        time_limit_ms,
    }
}

fn direct_runner() -> DirectRunner {
    DirectRunner::new()
}

#[test]
fn process_job_reads_files_runs_judge_core_and_keeps_submission_id() {
    let temp_dir = tempfile::tempdir().unwrap();
    let source_path = write_file(
        temp_dir.path(),
        "submissions/sub_ac_001/main.py",
        "print(input())",
    );
    let test_cases_path = write_file(
        temp_dir.path(),
        "problems/echo/testcases.json",
        r#"[{ "input": "hello\n", "expectedOutput": "hello\n" }]"#,
    );

    let runner = direct_runner();
    let result = process_job_with_runner(
        job("sub_ac_001", source_path, test_cases_path, 1_000),
        &runner,
    );

    assert_eq!(result.submission_id, "sub_ac_001");
    assert_eq!(result.result.verdict, Verdict::AC);
    assert_eq!(result.result.case_results.len(), 1);
}

#[test]
fn process_next_receives_jobs_in_fifo_order() {
    let temp_dir = tempfile::tempdir().unwrap();
    let source_one = write_file(
        temp_dir.path(),
        "submissions/sub_1/main.py",
        "print(input())",
    );
    let source_two = write_file(
        temp_dir.path(),
        "submissions/sub_2/main.py",
        "print(input())",
    );
    let tests_one = write_file(
        temp_dir.path(),
        "problems/one/testcases.json",
        r#"[{ "input": "one\n", "expectedOutput": "one\n" }]"#,
    );
    let tests_two = write_file(
        temp_dir.path(),
        "problems/two/testcases.json",
        r#"[{ "input": "two\n", "expectedOutput": "TWO\n" }]"#,
    );

    let mut queue = MockSqs::new(vec![
        job("sub_1", source_one, tests_one, 1_000),
        job("sub_2", source_two, tests_two, 1_000),
    ]);

    let runner = direct_runner();
    let first = process_next_with_runner(&mut queue, &runner).expect("first job should exist");
    let second = process_next_with_runner(&mut queue, &runner).expect("second job should exist");

    assert_eq!(first.submission_id, "sub_1");
    assert_eq!(first.result.verdict, Verdict::AC);
    assert_eq!(second.submission_id, "sub_2");
    assert_eq!(second.result.verdict, Verdict::WA);
    assert!(process_next_with_runner(&mut queue, &runner).is_none());
    assert!(queue.is_empty());
}

#[test]
fn mock_sqs_job_can_be_deserialized_from_json() {
    let temp_dir = tempfile::tempdir().unwrap();
    let source_path = write_file(
        temp_dir.path(),
        "submissions/sub_json_001/main.py",
        "print(input())",
    );
    let test_cases_path = write_file(
        temp_dir.path(),
        "problems/json/testcases.json",
        r#"[{ "input": "json\n", "expectedOutput": "json\n" }]"#,
    );

    let json = format!(
        r#"
        [
          {{
            "submissionId": "sub_json_001",
            "language": "python",
            "sourceCodePath": "{}",
            "testCasesPath": "{}",
            "timeLimitMs": 1000
          }}
        ]
    "#,
        source_path.to_string_lossy(),
        test_cases_path.to_string_lossy()
    );

    let jobs: Vec<JudgeJob> = serde_json::from_str(&json).expect("job JSON should parse");
    let mut queue = MockSqs::new(jobs);
    let runner = direct_runner();
    let result = process_next_with_runner(&mut queue, &runner).expect("job should exist");

    assert_eq!(result.submission_id, "sub_json_001");
    assert_eq!(result.result.verdict, Verdict::AC);
}

#[test]
fn returns_ie_when_referenced_file_is_missing() {
    let temp_dir = tempfile::tempdir().unwrap();
    let missing_source = temp_dir.path().join("missing/main.py");
    let test_cases_path = write_file(
        temp_dir.path(),
        "problems/echo/testcases.json",
        r#"[{ "input": "hello\n", "expectedOutput": "hello\n" }]"#,
    );

    let runner = direct_runner();
    let result = process_job_with_runner(
        job("sub_missing_source", missing_source, test_cases_path, 1_000),
        &runner,
    );

    assert_eq!(result.submission_id, "sub_missing_source");
    assert_eq!(result.result.verdict, Verdict::IE);
    assert!(result.result.case_results.is_empty());
}
