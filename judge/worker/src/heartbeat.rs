use std::{
    future::Future,
    pin::Pin,
    sync::{
        Arc,
        atomic::{AtomicBool, Ordering},
    },
    time::Duration,
};

pub type HeartbeatFuture<'a> = Pin<Box<dyn Future<Output = Result<(), String>> + Send + 'a>>;

pub trait VisibilityExtender: Clone + Send + Sync + 'static {
    fn extend_visibility<'a>(
        &'a self,
        receipt_handle: &'a str,
        seconds: i32,
    ) -> HeartbeatFuture<'a>;
}

// Runs until the caller has durably handled the message. A failed heartbeat is
// intentionally non-fatal: duplicate delivery is safe, whereas stopping the
// worker would make the failure harder to recover from.
pub fn spawn_visibility_heartbeat<Q: VisibilityExtender>(
    queue: Q,
    receipt_handle: String,
    completed: Arc<AtomicBool>,
    interval: Duration,
    visibility_timeout_seconds: i32,
) -> tokio::task::JoinHandle<()> {
    tokio::spawn(async move {
        while !completed.load(Ordering::Acquire) {
            tokio::time::sleep(interval).await;
            if completed.load(Ordering::Acquire) {
                break;
            }
            if let Err(error) = queue
                .extend_visibility(&receipt_handle, visibility_timeout_seconds)
                .await
            {
                eprintln!("failed to extend SQS visibility; message may be redelivered: {error}");
            }
        }
    })
}
