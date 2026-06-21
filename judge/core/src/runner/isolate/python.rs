use std::collections::HashMap;
use std::fs;
use std::path::Path;
use std::process::{Command, id};
use std::sync::atomic::{AtomicU32, Ordering};

use crate::runner::{ExecutionOutput, RunRequest};

const MEMORY_LIMIT_KB: u32 = 262_144;
static NEXT_BOX_ID: AtomicU32 = AtomicU32::new(0);

pub fn exec(request: RunRequest<'_>) -> Result<ExecutionOutput, String> {
    let box_id = reserve_box_id()?;
    let box_id_arg = format!("--box-id={}", box_id);
    let workspace = tempfile::tempdir().map_err(|error| error.to_string())?;
    let source_path = workspace.path().join("main.py");
    let stdin_path = workspace.path().join("stdin.txt");
    let meta_path = workspace.path().join("meta.txt");

    fs::write(&source_path, request.source_code).map_err(|error| error.to_string())?;
    fs::write(&stdin_path, request.input).map_err(|error| error.to_string())?;

    let init_output = Command::new("isolate")
        .arg(&box_id_arg)
        .arg("--init")
        .output()
        .map_err(|error| format!("failed to execute isolate --init: {}", error))?;

    let cleanup = Cleanup::new(box_id);

    if !init_output.status.success() {
        return Err(format!(
            "isolate --init failed: {}",
            String::from_utf8_lossy(&init_output.stderr).trim()
        ));
    }

    let box_root = String::from_utf8_lossy(&init_output.stdout)
        .trim()
        .to_string();
    if box_root.is_empty() {
        return Err(String::from("isolate --init did not return box path"));
    }

    let sandbox_box_dir = Path::new(&box_root).join("box");
    fs::copy(&source_path, sandbox_box_dir.join("main.py"))
        .map_err(|error| format!("failed to copy source code into isolate box: {}", error))?;
    fs::copy(&stdin_path, sandbox_box_dir.join("stdin.txt"))
        .map_err(|error| format!("failed to copy stdin into isolate box: {}", error))?;

    let time_limit_secs = format!("{:.3}", request.time_limit_ms as f64 / 1_000.0);
    let run_output = Command::new("isolate")
        .arg(&box_id_arg)
        .arg("--run")
        .arg(format!("--meta={}", meta_path.display()))
        .arg(format!("--time={}", time_limit_secs))
        .arg(format!("--wall-time={}", time_limit_secs))
        .arg(format!("--mem={}", MEMORY_LIMIT_KB))
        .arg("--stdin=stdin.txt")
        .arg("--stdout=stdout.txt")
        .arg("--stderr=stderr.txt")
        .arg("--")
        .arg("/usr/bin/python3")
        .arg("/box/main.py")
        .output()
        .map_err(|error| format!("failed to execute isolate --run: {}", error))?;

    let stdout = read_optional_file(&sandbox_box_dir.join("stdout.txt"))?;
    let stderr = read_optional_file(&sandbox_box_dir.join("stderr.txt"))?;
    let meta = read_meta(&meta_path)?;
    let timed_out = matches!(meta.get("status").map(String::as_str), Some("TO"));
    let runtime_ms = parse_runtime_ms(&meta);
    let status_success = run_output.status.success() && !timed_out;

    let isolate_stderr = String::from_utf8_lossy(&run_output.stderr)
        .trim()
        .to_string();
    if !timed_out && !run_output.status.success() && stderr.is_empty() && !isolate_stderr.is_empty()
    {
        return Err(format!("isolate --run failed: {}", isolate_stderr));
    }

    drop(cleanup);

    Ok(ExecutionOutput {
        stdout,
        stderr,
        status_success,
        runtime_ms,
        timed_out,
    })
}

fn reserve_box_id() -> Result<u32, String> {
    let base = id() % 1_000;

    for _ in 0..1_000 {
        let offset = NEXT_BOX_ID.fetch_add(1, Ordering::Relaxed) % 1_000;
        let box_id = (base + offset) % 1_000;
        let box_id_arg = format!("--box-id={}", box_id);

        let cleanup_output = Command::new("isolate")
            .arg(&box_id_arg)
            .arg("--cleanup")
            .output()
            .map_err(|error| format!("failed to execute isolate --cleanup: {}", error))?;

        if cleanup_output.status.success() || cleanup_output.status.code() == Some(1) {
            return Ok(box_id);
        }
    }

    Err(String::from("failed to reserve isolate box id"))
}

fn read_optional_file(path: &Path) -> Result<String, String> {
    match fs::read_to_string(path) {
        Ok(contents) => Ok(contents),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(String::new()),
        Err(error) => Err(error.to_string()),
    }
}

fn read_meta(path: &Path) -> Result<HashMap<String, String>, String> {
    let meta_text = read_optional_file(path)?;
    let mut meta = HashMap::new();

    for line in meta_text.lines() {
        if let Some((key, value)) = line.split_once(':') {
            meta.insert(key.trim().to_string(), value.trim().to_string());
        }
    }

    Ok(meta)
}

fn parse_runtime_ms(meta: &HashMap<String, String>) -> u128 {
    let seconds = meta
        .get("time-wall")
        .or_else(|| meta.get("time"))
        .and_then(|value| value.parse::<f64>().ok())
        .unwrap_or(0.0);

    (seconds * 1_000.0) as u128
}

struct Cleanup {
    box_id: u32,
}

impl Cleanup {
    fn new(box_id: u32) -> Self {
        Self { box_id }
    }
}

impl Drop for Cleanup {
    fn drop(&mut self) {
        let _ = Command::new("isolate")
            .arg(format!("--box-id={}", self.box_id))
            .arg("--cleanup")
            .output();
    }
}
