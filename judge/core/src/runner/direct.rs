use crate::models::Language;
use crate::runner::{ExecutionOutput, RunRequest, Runner, python};

pub struct DirectRunner;

impl DirectRunner {
    pub fn new() -> Self {
        Self
    }
}

impl Runner for DirectRunner {
    fn run(&self, request: RunRequest<'_>) -> Result<ExecutionOutput, String> {
        match request.language {
            Language::Python => {
                python::exec_py(request.source_code, request.input, request.time_limit_ms)
            }
        }
    }
}
