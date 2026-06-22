use std::{env, future::Future, pin::Pin};

use aws_sdk_s3::{
    Client, Config,
    config::{Credentials, Region},
};
use aws_smithy_runtime::client::http::hyper_014::HyperClientBuilder;
use hyper_rustls::HttpsConnectorBuilder;

const DEFAULT_REGION: &str = "ap-northeast-1";
const DEFAULT_ENDPOINT_URL: &str = "http://localhost:4566";
const DEFAULT_BUCKET_NAME: &str = "judge-storage";

pub trait ArtifactStore {
    fn read_source_code<'a>(
        &'a self,
        source_code_key: &'a str,
    ) -> Pin<Box<dyn Future<Output = Result<String, String>> + Send + 'a>>;

    fn read_test_cases<'a>(
        &'a self,
        test_cases_key: &'a str,
    ) -> Pin<Box<dyn Future<Output = Result<String, String>> + Send + 'a>>;
}

#[derive(Debug)]
pub struct S3Storage {
    client: Client,
    bucket: String,
}

#[derive(Debug)]
pub struct S3StorageConfig {
    region: String,
    endpoint_url: String,
    bucket: String,
}

impl S3StorageConfig {
    pub fn from_env() -> Self {
        let region = env::var("AWS_DEFAULT_REGION").unwrap_or_else(|_| DEFAULT_REGION.to_string());
        let endpoint_url =
            env::var("AWS_ENDPOINT_URL").unwrap_or_else(|_| DEFAULT_ENDPOINT_URL.to_string());
        let bucket =
            env::var("JUDGE_STORAGE_BUCKET").unwrap_or_else(|_| DEFAULT_BUCKET_NAME.to_string());

        Self {
            region,
            endpoint_url,
            bucket,
        }
    }
}

impl S3Storage {
    pub async fn connect(storage_config: S3StorageConfig) -> Result<Self, String> {
        let https_connector = HttpsConnectorBuilder::new()
            .with_webpki_roots()
            .https_or_http()
            .enable_http1()
            .enable_http2()
            .build();
        let http_client = HyperClientBuilder::new().build(https_connector);

        let sdk_config = Config::builder()
            .behavior_version_latest()
            .region(Region::new(storage_config.region))
            .endpoint_url(storage_config.endpoint_url)
            .credentials_provider(Credentials::new("test", "test", None, None, "localstack"))
            .http_client(http_client)
            .force_path_style(true)
            .build();

        Ok(Self {
            client: Client::from_conf(sdk_config),
            bucket: storage_config.bucket,
        })
    }

    async fn read_utf8(&self, key: &str) -> Result<String, String> {
        let response = self
            .client
            .get_object()
            .bucket(&self.bucket)
            .key(key)
            .send()
            .await
            .map_err(|error| format!("failed to fetch s3://{}/{}: {}", self.bucket, key, error))?;

        let body = response
            .body
            .collect()
            .await
            .map_err(|error| format!("failed to read s3://{}/{} body: {}", self.bucket, key, error))?;

        String::from_utf8(body.into_bytes().to_vec())
            .map_err(|error| format!("s3://{}/{} is not valid UTF-8: {}", self.bucket, key, error))
    }
}

impl ArtifactStore for S3Storage {
    fn read_source_code<'a>(
        &'a self,
        source_code_key: &'a str,
    ) -> Pin<Box<dyn Future<Output = Result<String, String>> + Send + 'a>> {
        Box::pin(async move { self.read_utf8(source_code_key).await })
    }

    fn read_test_cases<'a>(
        &'a self,
        test_cases_key: &'a str,
    ) -> Pin<Box<dyn Future<Output = Result<String, String>> + Send + 'a>> {
        Box::pin(async move { self.read_utf8(test_cases_key).await })
    }
}
