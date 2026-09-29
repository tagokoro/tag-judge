use std::{env, future::Future, pin::Pin};

use judge_core::models::Verdict;
use native_tls::TlsConnector;
use postgres_native_tls::MakeTlsConnector;
use tokio::sync::Mutex;
use tokio_postgres::Client;

use crate::models::WorkerResult;

type SinkFuture<'a, T> = Pin<Box<dyn Future<Output = Result<T, String>> + Send + 'a>>;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum StartOutcome {
    Started,
    AlreadyTerminal,
}

pub trait ResultSink: Send + Sync {
    fn mark_running<'a>(&'a self, submission_id: &'a str) -> SinkFuture<'a, StartOutcome>;
    fn complete<'a>(&'a self, result: &'a WorkerResult) -> SinkFuture<'a, ()>;
}

#[derive(Debug)]
pub struct PostgresResultSinkConfig {
    database_url: String,
}
impl PostgresResultSinkConfig {
    pub fn new(database_url: impl Into<String>) -> Result<Self, String> {
        let database_url = database_url.into();
        if database_url.trim().is_empty() {
            return Err("judge database URL must not be empty".to_string());
        }
        Ok(Self { database_url })
    }
    pub fn from_env() -> Result<Self, String> {
        Self::new(
            env::var("JUDGE_DATABASE_URL")
                .or_else(|_| env::var("DATABASE_URL"))
                .map_err(|_| "JUDGE_DATABASE_URL or DATABASE_URL must be set".to_string())?,
        )
    }
}

pub struct PostgresResultSink {
    client: Mutex<Client>,
}
impl PostgresResultSink {
    pub async fn connect(config: PostgresResultSinkConfig) -> Result<Self, String> {
        let tls = TlsConnector::builder()
            .build()
            .map_err(|e| format!("build PostgreSQL TLS connector: {e}"))?;
        let (client, connection) =
            tokio_postgres::connect(&config.database_url, MakeTlsConnector::new(tls))
                .await
                .map_err(|e| format!("connect to judge database: {e}"))?;
        tokio::spawn(async move {
            if let Err(error) = connection.await {
                eprintln!("judge database connection closed: {error}");
            }
        });
        Ok(Self {
            client: Mutex::new(client),
        })
    }
}

impl ResultSink for PostgresResultSink {
    fn mark_running<'a>(&'a self, submission_id: &'a str) -> SinkFuture<'a, StartOutcome> {
        Box::pin(async move {
            let id = parse_submission_id(submission_id)?;
            let mut client = self.client.lock().await;
            let tx = client
                .transaction()
                .await
                .map_err(|e| format!("begin mark-running transaction: {e}"))?;
            let row = tx
                .query_opt(
                    "SELECT status FROM submissions WHERE id = $1 FOR UPDATE",
                    &[&id],
                )
                .await
                .map_err(|e| format!("lock submission {id}: {e}"))?
                .ok_or_else(|| format!("submission {id} does not exist"))?;
            let status: String = row.get("status");
            if is_terminal_status(&status) {
                tx.commit()
                    .await
                    .map_err(|e| format!("commit terminal check: {e}"))?;
                return Ok(StartOutcome::AlreadyTerminal);
            }
            if status != "pending" && status != "running" {
                return Err(format!("submission {id} has unsupported status {status}"));
            }
            tx.execute(
                "UPDATE submissions SET status = 'running' WHERE id = $1",
                &[&id],
            )
            .await
            .map_err(|e| format!("mark submission {id} running: {e}"))?;
            tx.commit()
                .await
                .map_err(|e| format!("commit running status: {e}"))?;
            Ok(StartOutcome::Started)
        })
    }

    fn complete<'a>(&'a self, result: &'a WorkerResult) -> SinkFuture<'a, ()> {
        Box::pin(async move {
            let id = parse_submission_id(&result.submission_id)?;
            if result.score < 0 {
                return Err("submission result score must not be negative".to_string());
            }
            let status = status_for_verdict(result.result.verdict);
            let mut client = self.client.lock().await;
            let tx = client
                .transaction()
                .await
                .map_err(|e| format!("begin result transaction: {e}"))?;
            let row = tx.query_opt("SELECT problem_id, status, test_cases_key FROM submissions WHERE id = $1 FOR UPDATE", &[&id]).await.map_err(|e| format!("lock submission {id}: {e}"))?.ok_or_else(|| format!("submission {id} does not exist"))?;
            let current: String = row.get("status");
            if is_terminal_status(&current) {
                tx.commit()
                    .await
                    .map_err(|e| format!("commit duplicate check: {e}"))?;
                return Ok(());
            }
            if current != "pending" && current != "running" {
                return Err(format!("submission {id} has unsupported status {current}"));
            }
            let problem_id: i64 = row.get("problem_id");
            let key: String = row.get("test_cases_key");
            if key != result.test_cases_key {
                return Err(format!("submission {id} test cases key mismatch"));
            }
            tx.execute(
                "DELETE FROM submission_results WHERE submission_id = $1",
                &[&id],
            )
            .await
            .map_err(|e| format!("replace submission {id} results: {e}"))?;
            for case in &result.result.case_results {
                let runtime = i32::try_from(case.runtime_ms)
                    .map_err(|_| format!("case {} runtime is too large", case.case_index))?;
                let inserted = tx.execute("INSERT INTO submission_results (submission_id, test_case_id, status, execution_time_ms, memory_kb, stdout, stderr) SELECT $1, tc.id, $4, $5, NULL, $6, $7 FROM test_cases tc WHERE tc.id = $2 AND tc.problem_id = $3", &[&id, &case.test_case_id, &problem_id, &status_for_verdict(case.verdict), &runtime, &case.stdout, &case.stderr]).await.map_err(|e| format!("insert case result: {e}"))?;
                if inserted != 1 {
                    return Err(format!(
                        "test case {} does not belong to submission {id} problem",
                        case.test_case_id
                    ));
                }
            }
            tx.execute(
                "UPDATE submissions SET status = $2, score = $3, judged_at = now() WHERE id = $1",
                &[&id, &status, &result.score],
            )
            .await
            .map_err(|e| format!("finalize submission {id}: {e}"))?;
            tx.commit()
                .await
                .map_err(|e| format!("commit submission {id} result: {e}"))?;
            Ok(())
        })
    }
}

fn parse_submission_id(value: &str) -> Result<i64, String> {
    let id = value
        .parse::<i64>()
        .map_err(|_| format!("submission id must be a positive integer: {value}"))?;
    if id <= 0 {
        return Err(format!("submission id must be positive: {value}"));
    }
    Ok(id)
}
fn is_terminal_status(status: &str) -> bool {
    matches!(status, "ac" | "wa" | "tle" | "mle" | "re" | "ce" | "ie")
}
fn status_for_verdict(v: Verdict) -> &'static str {
    match v {
        Verdict::AC => "ac",
        Verdict::WA => "wa",
        Verdict::TLE => "tle",
        Verdict::RE => "re",
        Verdict::CE => "ce",
        Verdict::IE => "ie",
    }
}
