pub mod direct;
pub mod isolate;

use crate::models::Language;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RunnerKind {
    Direct,
    Isolate,
}

impl RunnerKind {
    pub fn default_for_current_os() -> Self {
        if cfg!(target_os = "linux") {
            Self::Isolate
        } else {
            Self::Direct
        }
    }
}

#[derive(Debug)]
pub struct ExecutionOutput {
    pub stdout: String,
    pub stderr: String,
    pub status_success: bool,
    pub runtime_ms: u128,
    pub timed_out: bool,
}

pub struct RunRequest<'a> {
    pub language: Language,
    pub source_code: &'a str,
    pub input: &'a str,
    pub time_limit_ms: u32,
}

pub trait Runner {
    fn run(&self, request: RunRequest<'_>) -> Result<ExecutionOutput, String>;
}

pub fn default_runner() -> Result<Box<dyn Runner>, String> {
    runner_for_kind(RunnerKind::default_for_current_os())
}

pub fn runner_for_kind(kind: RunnerKind) -> Result<Box<dyn Runner>, String> {
    match kind {
        RunnerKind::Direct => Ok(Box::new(direct::DirectRunner::new())),
        RunnerKind::Isolate => {
            isolate::IsolateRunner::new().map(|runner| Box::new(runner) as Box<dyn Runner>)
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{RunnerKind, runner_for_kind};

    #[test]
    fn default_runner_kind_matches_target_os() {
        #[cfg(target_os = "linux")]
        assert_eq!(RunnerKind::default_for_current_os(), RunnerKind::Isolate);

        #[cfg(not(target_os = "linux"))]
        assert_eq!(RunnerKind::default_for_current_os(), RunnerKind::Direct);
    }

    #[test]
    fn runner_factory_creates_requested_runner() {
        assert!(runner_for_kind(RunnerKind::Direct).is_ok());
    }
}
