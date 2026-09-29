use std::{collections::HashMap, future::Future, pin::Pin};

use judge_core::{
    models::{Language, Verdict},
    runner::direct::DirectRunner,
};
use judge_worker::{
    mock_sqs::MockSqs,
    models::JudgeJob,
    storage::ArtifactStore,
    worker::{process_job_with_runner, process_next_with_runner},
};

struct MemoryArtifactStore {
    objects: HashMap<String, String>,
}

impl MemoryArtifactStore {
    fn new(entries: Vec<(&str, &str)>) -> Self {
        let objects = entries
            .into_iter()
            .map(|(key, value)| (key.to_string(), value.to_string()))
            .collect();

        Self { objects }
    }

    fn read_object<'a>(
        &'a self,
        key: &'a str,
    ) -> Pin<Box<dyn Future<Output = Result<String, String>> + Send + 'a>> {
        Box::pin(async move {
            self.objects
                .get(key)
                .cloned()
                .ok_or_else(|| format!("missing object for key {}", key))
        })
    }
}

impl ArtifactStore for MemoryArtifactStore {
    fn read_source_code<'a>(
        &'a self,
        source_code_key: &'a str,
    ) -> Pin<Box<dyn Future<Output = Result<String, String>> + Send + 'a>> {
        self.read_object(source_code_key)
    }

    fn read_test_cases<'a>(
        &'a self,
        test_cases_key: &'a str,
    ) -> Pin<Box<dyn Future<Output = Result<String, String>> + Send + 'a>> {
        self.read_object(test_cases_key)
    }
}

fn job(
    submission_id: &str,
    source_code_key: &str,
    test_cases_key: &str,
    time_limit_ms: u32,
) -> JudgeJob {
    JudgeJob {
        submission_id: submission_id.to_string(),
        language: Language::Python,
        source_code_key: source_code_key.to_string(),
        test_cases_key: test_cases_key.to_string(),
        time_limit_ms,
    }
}

fn direct_runner() -> DirectRunner {
    DirectRunner::new()
}

#[tokio::test]
async fn process_job_reads_s3_objects_runs_judge_core_and_keeps_submission_id() {
    let store = MemoryArtifactStore::new(vec![
        ("submissions/sub_ac_001/main.py", "print(input())"),
        (
            "problems/42/testcases/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.json",
            r#"[{ "input": "hello\n", "expectedOutput": "hello\n", "testCaseId": 1, "score": 1 }]"#,
        ),
    ]);

    let runner = direct_runner();
    let result = process_job_with_runner(
        job(
            "sub_ac_001",
            "submissions/sub_ac_001/main.py",
            "problems/42/testcases/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.json",
            1_000,
        ),
        &store,
        &runner,
    )
    .await;

    assert_eq!(result.submission_id, "sub_ac_001");
    assert_eq!(result.result.verdict, Verdict::AC);
    assert_eq!(result.result.case_results.len(), 1);
}

#[tokio::test]
async fn process_next_receives_jobs_in_fifo_order() {
    let store = MemoryArtifactStore::new(vec![
        ("submissions/sub_1/main.py", "print(input())"),
        ("submissions/sub_2/main.py", "print(input())"),
        (
            "problems/prob_101/testcases.json",
            r#"[{ "input": "one\n", "expectedOutput": "one\n" }]"#,
        ),
        (
            "problems/prob_102/testcases.json",
            r#"[{ "input": "two\n", "expectedOutput": "TWO\n" }]"#,
        ),
    ]);

    let mut queue = MockSqs::new(vec![
        job(
            "sub_1",
            "submissions/sub_1/main.py",
            "problems/prob_101/testcases.json",
            1_000,
        ),
        job(
            "sub_2",
            "submissions/sub_2/main.py",
            "problems/prob_102/testcases.json",
            1_000,
        ),
    ]);

    let runner = direct_runner();
    let first = process_next_with_runner(&mut queue, &store, &runner)
        .await
        .expect("first job should exist");
    let second = process_next_with_runner(&mut queue, &store, &runner)
        .await
        .expect("second job should exist");

    assert_eq!(first.submission_id, "sub_1");
    assert_eq!(first.result.verdict, Verdict::AC);
    assert_eq!(second.submission_id, "sub_2");
    assert_eq!(second.result.verdict, Verdict::WA);
    assert!(
        process_next_with_runner(&mut queue, &store, &runner)
            .await
            .is_none()
    );
    assert!(queue.is_empty());
}

#[test]
fn mock_sqs_job_can_be_deserialized_from_json() {
    let json = r#"
        [
          {
            "submissionId": "sub_json_001",
            "language": "python",
            "sourceCodeKey": "submissions/sub_json_001/main.py",
            "testCasesKey": "problems/prob_201/testcases.json",
            "timeLimitMs": 1000
          }
        ]
    "#;

    let jobs: Vec<JudgeJob> = serde_json::from_str(json).expect("job JSON should parse");
    let job = jobs.first().expect("job should exist");

    assert_eq!(job.submission_id, "sub_json_001");
    assert_eq!(job.source_code_key, "submissions/sub_json_001/main.py");
    assert_eq!(job.test_cases_key, "problems/prob_201/testcases.json");
}

#[tokio::test]
async fn returns_ie_when_referenced_s3_object_is_missing() {
    let store = MemoryArtifactStore::new(vec![(
        "problems/prob_001/testcases.json",
        r#"[{ "input": "hello\n", "expectedOutput": "hello\n" }]"#,
    )]);

    let runner = direct_runner();
    let result = process_job_with_runner(
        job(
            "sub_missing_source",
            "submissions/missing/main.py",
            "problems/prob_001/testcases.json",
            1_000,
        ),
        &store,
        &runner,
    )
    .await;

    assert_eq!(result.submission_id, "sub_missing_source");
    assert_eq!(result.result.verdict, Verdict::IE);
    assert!(result.result.case_results.is_empty());
}

#[tokio::test]
async fn returns_ie_when_test_cases_json_is_invalid() {
    let store = MemoryArtifactStore::new(vec![
        ("submissions/sub_invalid/main.py", "print(input())"),
        ("problems/prob_001/testcases.json", "{not-json}"),
    ]);

    let runner = direct_runner();
    let result = process_job_with_runner(
        job(
            "sub_invalid",
            "submissions/sub_invalid/main.py",
            "problems/prob_001/testcases.json",
            1_000,
        ),
        &store,
        &runner,
    )
    .await;

    assert_eq!(result.result.verdict, Verdict::IE);
    assert!(
        result
            .result
            .message
            .expect("message should exist")
            .contains("failed to parse test cases JSON")
    );
}
