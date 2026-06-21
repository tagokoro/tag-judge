use crate::runner::{ExecutionOutput, RunRequest, Runner};

pub struct IsolateRunner {
    _private: (),
}

impl IsolateRunner {
    pub fn new() -> Result<Self, String> {
        Err(String::from(
            "IsolateRunner is selected for this OS, but isolate execution is not implemented yet",
        ))
    }
}

impl Runner for IsolateRunner {
    fn run(&self, _request: RunRequest<'_>) -> Result<ExecutionOutput, String> {
        Err(String::from(
            "IsolateRunner execution is not implemented yet",
        ))
    }
}
