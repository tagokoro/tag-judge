use crate::compare::output_matches;
use crate::models::{CaseResult, JudgeRequest, JudgeResult, Language, TestCase, Verdict};
use crate::runner::python::{ExecutionOutput, exec_py};

pub fn run(request: JudgeRequest) -> JudgeResult {
    if request.test_cases.is_empty() {
        return JudgeResult {
            verdict: Verdict::IE,
            case_results: vec![],
            message: Some(String::from("test_cases must contain at least one case")),
        };
    }

    let mut case_results = Vec::new();

    for (case_index, test_case) in request.test_cases.iter().enumerate() {
        let execution = match request.language {
            Language::Python => exec_py(
                &request.source_code,
                &test_case.input,
                request.time_limit_ms,
            ),
        };

        let case_result = match execution {
            Ok(output) => build_case_result(case_index, test_case, output),
            Err(message) => CaseResult {
                case_index,
                verdict: Verdict::IE,
                runtime_ms: 0,
                stdout: String::new(),
                stderr: String::new(),
                message: Some(message),
            },
        };

        let should_stop = case_result.verdict != Verdict::AC;
        case_results.push(case_result);

        if should_stop {
            break;
        }
    }

    let verdict = aggregate_verdict(&case_results);
    let message = match verdict {
        Verdict::AC => Some(String::from("Accepted")),
        _ => case_results
            .last()
            .and_then(|result| result.message.clone()),
    };

    JudgeResult {
        verdict,
        case_results,
        message,
    }
}

fn build_case_result(
    case_index: usize,
    test_case: &TestCase,
    output: ExecutionOutput,
) -> CaseResult {
    if output.timed_out {
        return CaseResult {
            case_index,
            verdict: Verdict::TLE,
            runtime_ms: output.runtime_ms,
            stdout: output.stdout,
            stderr: output.stderr,
            message: Some(format!("Time limit exceeded on case {}", case_index)),
        };
    }

    if !output.status_success {
        return CaseResult {
            case_index,
            verdict: Verdict::RE,
            runtime_ms: output.runtime_ms,
            stdout: output.stdout,
            stderr: output.stderr,
            message: Some(format!("Runtime error on case {}", case_index)),
        };
    }

    if output_matches(&output.stdout, &test_case.expected_output) {
        CaseResult {
            case_index,
            verdict: Verdict::AC,
            runtime_ms: output.runtime_ms,
            stdout: output.stdout,
            stderr: output.stderr,
            message: Some(format!("Accepted on case {}", case_index)),
        }
    } else {
        CaseResult {
            case_index,
            verdict: Verdict::WA,
            runtime_ms: output.runtime_ms,
            stdout: output.stdout,
            stderr: output.stderr,
            message: Some(format!("Wrong answer on case {}", case_index)),
        }
    }
}

fn aggregate_verdict(case_results: &[CaseResult]) -> Verdict {
    if case_results
        .iter()
        .all(|result| result.verdict == Verdict::AC)
    {
        return Verdict::AC;
    }

    for verdict in [
        Verdict::IE,
        Verdict::CE,
        Verdict::TLE,
        Verdict::RE,
        Verdict::WA,
    ] {
        if case_results.iter().any(|result| result.verdict == verdict) {
            return verdict;
        }
    }

    Verdict::IE
}
