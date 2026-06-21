use crate::models::Language;
use crate::runner::{ExecutionOutput, RunRequest, Runner};

mod python;

pub struct IsolateRunner {
    _private: (),
}

impl IsolateRunner {
    pub fn new() -> Result<Self, String> {
        Ok(Self { _private: () })
    }
}

impl Runner for IsolateRunner {
    fn run(&self, request: RunRequest<'_>) -> Result<ExecutionOutput, String> {
        match request.language {
            Language::Python => python::exec(request),
        }
    }
}
