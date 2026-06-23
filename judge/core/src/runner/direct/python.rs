use std::fs::write;
use std::io::Write;
use std::process::{Command, Stdio};
use std::thread;
use std::time::{Duration, Instant};

use crate::runner::ExecutionOutput;

pub fn exec(source_code: &str, input: &str, time_limit_ms: u32) -> Result<ExecutionOutput, String> {
    let work_dir = tempfile::tempdir().map_err(|error| error.to_string())?;
    let source_path = work_dir.path().join("main.py");
    write(&source_path, source_code).map_err(|error| error.to_string())?;

    let mut child = Command::new("python3")
        .arg(&source_path)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|error| error.to_string())?;

    let mut stdin = child
        .stdin
        .take()
        .ok_or_else(|| String::from("failed to open stdin"))?;
    stdin
        .write_all(input.as_bytes())
        .map_err(|error| error.to_string())?;
    drop(stdin);

    let start = Instant::now();
    let time_limit = Duration::from_millis(time_limit_ms as u64);
    let mut timed_out = false;

    loop {
        if child
            .try_wait()
            .map_err(|error| error.to_string())?
            .is_some()
        {
            break;
        }

        if start.elapsed() >= time_limit {
            timed_out = true;
            child.kill().map_err(|error| error.to_string())?;
            break;
        }

        thread::sleep(Duration::from_millis(5));
    }

    let output = child
        .wait_with_output()
        .map_err(|error| error.to_string())?;
    let runtime_ms = start.elapsed().as_millis();

    Ok(ExecutionOutput {
        stdout: String::from_utf8_lossy(&output.stdout).to_string(),
        stderr: String::from_utf8_lossy(&output.stderr).to_string(),
        status_success: output.status.success(),
        runtime_ms,
        timed_out,
    })
}
