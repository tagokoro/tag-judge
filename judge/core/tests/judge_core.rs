use judge_core::{
    compare::output_matches,
    judge,
    models::{JudgeRequest, Language, TestCase, Verdict},
    runner::direct::DirectRunner,
};

fn request(source_code: &str, test_cases: Vec<TestCase>) -> JudgeRequest {
    JudgeRequest {
        language: Language::Python,
        source_code: source_code.to_string(),
        time_limit_ms: 1_000,
        test_cases,
    }
}

fn case(input: &str, expected_output: &str) -> TestCase {
    TestCase {
        input: input.to_string(),
        expected_output: expected_output.to_string(),
    }
}

fn run_direct(request: JudgeRequest) -> judge_core::models::JudgeResult {
    let runner = DirectRunner::new();
    judge::run_with_runner(request, &runner)
}

#[test]
fn accepts_all_cases_when_outputs_match() {
    let result = run_direct(request(
        "print(input())",
        vec![case("hello\n", "hello\n"), case("world\n", "world\n")],
    ));

    assert_eq!(result.verdict, Verdict::AC);
    assert_eq!(result.case_results.len(), 2);
    assert_eq!(result.case_results[0].verdict, Verdict::AC);
    assert_eq!(result.case_results[1].verdict, Verdict::AC);
}

#[test]
fn returns_wa_and_stops_at_first_failed_case() {
    let result = run_direct(request(
        "print(input())",
        vec![
            case("hello\n", "hello\n"),
            case("world\n", "WORLD\n"),
            case("ignored\n", "ignored\n"),
        ],
    ));

    assert_eq!(result.verdict, Verdict::WA);
    assert_eq!(result.case_results.len(), 2);
    assert_eq!(result.case_results[1].case_index, 1);
    assert_eq!(result.case_results[1].verdict, Verdict::WA);
}

#[test]
fn returns_re_when_python_process_fails() {
    let result = run_direct(request(
        "raise RuntimeError('boom')",
        vec![case("\n", "\n")],
    ));

    assert_eq!(result.verdict, Verdict::RE);
    assert_eq!(result.case_results.len(), 1);
    assert_eq!(result.case_results[0].verdict, Verdict::RE);
    assert!(result.case_results[0].stderr.contains("RuntimeError"));
}

#[test]
fn returns_tle_when_time_limit_is_exceeded() {
    let mut judge_request = request("while True:\n    pass", vec![case("\n", "\n")]);
    judge_request.time_limit_ms = 50;

    let result = run_direct(judge_request);

    assert_eq!(result.verdict, Verdict::TLE);
    assert_eq!(result.case_results.len(), 1);
    assert_eq!(result.case_results[0].verdict, Verdict::TLE);
}

#[test]
fn returns_ie_when_test_cases_are_empty() {
    let result = run_direct(request("print(1)", vec![]));

    assert_eq!(result.verdict, Verdict::IE);
    assert!(result.case_results.is_empty());
}

#[test]
fn compare_ignores_only_trailing_whitespace() {
    assert!(output_matches("hello\n", "hello"));
    assert!(output_matches("hello   \n", "hello"));
    assert!(!output_matches("hello  world\n", "hello world\n"));
}
