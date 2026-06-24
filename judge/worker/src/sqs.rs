use std::env;

use aws_config::BehaviorVersion;
use aws_sdk_sqs::{
    Client, Config,
    config::{Credentials, Region},
};
use aws_smithy_runtime::client::http::hyper_014::HyperClientBuilder;
use hyper_rustls::HttpsConnectorBuilder;

use crate::models::JudgeJob;

const DEFAULT_REGION: &str = "ap-northeast-1";
const DEFAULT_QUEUE_NAME: &str = "judge-submissions";
const MAX_MESSAGES_PER_POLL: i32 = 1;
const WAIT_TIME_SECONDS: i32 = 5;

#[derive(Debug)]
pub struct ReceivedJudgeJob {
    pub job: JudgeJob,
    pub receipt_handle: String,
}

#[derive(Debug)]
pub struct SqsQueue {
    client: Client,
    queue_url: String,
}

#[derive(Debug)]
pub struct SqsQueueConfig {
    region: String,
    endpoint_url: Option<String>,
    queue_name: String,
}

impl SqsQueueConfig {
    pub fn from_env() -> Self {
        let region = env::var("AWS_DEFAULT_REGION").unwrap_or_else(|_| DEFAULT_REGION.to_string());
        let endpoint_url = env::var("AWS_ENDPOINT_URL")
            .ok()
            .filter(|value| !value.trim().is_empty());
        let queue_name = env::var("JUDGE_SUBMISSIONS_QUEUE_NAME")
            .unwrap_or_else(|_| DEFAULT_QUEUE_NAME.to_string());

        Self {
            region,
            endpoint_url,
            queue_name,
        }
    }
}

impl SqsQueue {
    pub async fn connect(queue_config: SqsQueueConfig) -> Result<Self, String> {
        let client = match queue_config.endpoint_url {
            Some(endpoint_url) => localstack_client(&queue_config.region, endpoint_url),
            None => aws_client(&queue_config.region).await,
        };

        let queue_url = client
            .get_queue_url()
            .queue_name(queue_config.queue_name)
            .send()
            .await
            .map_err(|error| format!("failed to resolve SQS queue URL: {}", error))?
            .queue_url
            .ok_or_else(|| "SQS get-queue-url response did not include queueUrl".to_string())?;

        Ok(Self { client, queue_url })
    }

    pub async fn receive_job(&self) -> Result<Option<ReceivedJudgeJob>, String> {
        let response = self
            .client
            .receive_message()
            .queue_url(&self.queue_url)
            .max_number_of_messages(MAX_MESSAGES_PER_POLL)
            .wait_time_seconds(WAIT_TIME_SECONDS)
            .send()
            .await
            .map_err(|error| format!("failed to receive SQS message: {}", error))?;

        let Some(message) = response.messages().first() else {
            return Ok(None);
        };

        let body = message
            .body()
            .ok_or_else(|| "received SQS message did not include body".to_string())?;
        let receipt_handle = message
            .receipt_handle()
            .ok_or_else(|| "received SQS message did not include receiptHandle".to_string())?
            .to_string();
        let job = serde_json::from_str::<JudgeJob>(body)
            .map_err(|error| format!("failed to parse SQS message body as JudgeJob: {}", error))?;

        Ok(Some(ReceivedJudgeJob {
            job,
            receipt_handle,
        }))
    }

    pub async fn delete_message(&self, receipt_handle: &str) -> Result<(), String> {
        self.client
            .delete_message()
            .queue_url(&self.queue_url)
            .receipt_handle(receipt_handle)
            .send()
            .await
            .map_err(|error| format!("failed to delete SQS message: {}", error))?;

        Ok(())
    }
}

fn localstack_client(region: &str, endpoint_url: String) -> Client {
    // LocalStack uses an HTTP endpoint and accepts dummy static credentials.
    let https_connector = HttpsConnectorBuilder::new()
        .with_webpki_roots()
        .https_or_http()
        .enable_http1()
        .enable_http2()
        .build();
    let http_client = HyperClientBuilder::new().build(https_connector);

    let sdk_config = Config::builder()
        .behavior_version_latest()
        .region(Region::new(region.to_string()))
        .endpoint_url(endpoint_url)
        .credentials_provider(Credentials::new("test", "test", None, None, "localstack"))
        .http_client(http_client)
        .build();

    Client::from_conf(sdk_config)
}

async fn aws_client(region: &str) -> Client {
    let sdk_config = aws_config::defaults(BehaviorVersion::latest())
        .region(Region::new(region.to_string()))
        .load()
        .await;

    Client::new(&sdk_config)
}
