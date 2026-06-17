use std::collections::VecDeque;

use crate::models::JudgeJob;

#[derive(Debug)]
pub struct MockSqs {
    jobs: VecDeque<JudgeJob>,
}

impl MockSqs {
    pub fn new(jobs: Vec<JudgeJob>) -> Self {
        Self {
            jobs: VecDeque::from(jobs),
        }
    }

    pub fn receive_message(&mut self) -> Option<JudgeJob> {
        self.jobs.pop_front()
    }

    pub fn is_empty(&self) -> bool {
        self.jobs.is_empty()
    }
}
