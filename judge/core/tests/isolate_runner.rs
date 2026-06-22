#[cfg(target_os = "linux")]
mod tests {
    use std::path::Path;
    use std::process::Command;
    use std::process::id;

    use judge_core::{
        judge,
        models::{JudgeRequest, Language, TestCase, Verdict},
        runner::isolate::IsolateRunner,
    };

    fn isolate_available() -> bool {
        if !Command::new("isolate")
            .arg("--version")
            .output()
            .map(|output| output.status.success())
            .unwrap_or(false)
            || !Path::new("/usr/bin/python3").exists()
        {
            return false;
        }

        isolate_cgroups_available()
    }

    fn isolate_cgroups_available() -> bool {
        for offset in 0..16 {
            let box_id = 900 + ((id() + offset) % 100);
            let box_id_arg = format!("--box-id={}", box_id);
            let init = Command::new("isolate")
                .arg("--cg")
                .arg(&box_id_arg)
                .arg("--init")
                .output();

            let Ok(output) = init else {
                return false;
            };

            if output.status.success() {
                let _ = Command::new("isolate")
                    .arg("--cg")
                    .arg(&box_id_arg)
                    .arg("--cleanup")
                    .output();
                return true;
            }
        }

        false
    }

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

    fn run_isolate(request: JudgeRequest) -> judge_core::models::JudgeResult {
        let runner = IsolateRunner::new().expect("isolate runner should be created");
        judge::run_with_runner(request, &runner)
    }

    #[test]
    fn accepts_all_cases_when_outputs_match() {
        if !isolate_available() {
            eprintln!("skipped: isolate or /usr/bin/python3 is unavailable");
            return;
        }

        let result = run_isolate(request(
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
        if !isolate_available() {
            eprintln!("skipped: isolate or /usr/bin/python3 is unavailable");
            return;
        }

        let result = run_isolate(request(
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
        if !isolate_available() {
            eprintln!("skipped: isolate or /usr/bin/python3 is unavailable");
            return;
        }

        let result = run_isolate(request(
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
        if !isolate_available() {
            eprintln!("skipped: isolate or /usr/bin/python3 is unavailable");
            return;
        }

        let mut judge_request = request("while True:\n    pass", vec![case("\n", "\n")]);
        judge_request.time_limit_ms = 50;

        let result = run_isolate(judge_request);

        assert_eq!(result.verdict, Verdict::TLE);
        assert_eq!(result.case_results.len(), 1);
        assert_eq!(result.case_results[0].verdict, Verdict::TLE);
    }
}
