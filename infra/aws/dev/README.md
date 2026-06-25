# AWS dev judge worker

This Terraform stack creates the smallest AWS environment needed to verify that the Rust judge worker can consume a job from SQS, read artifacts from S3, and run `judge-core` on EC2.

## Resources

- S3 bucket for source code and test case artifacts
- SQS queue and dead-letter queue
- IAM role and instance profile for the worker EC2 instance
- EC2 instance running `judge-worker` as a systemd service
- Security group with no inbound rules

The worker instance installs build dependencies from apt and builds `ioi/isolate` from the pinned `isolate_git_ref` Terraform variable. Ubuntu's default apt repositories do not provide the `isolate` package used by `judge-core`.

The EC2 instance is intended to be accessed through AWS Systems Manager Session Manager, not SSH.

## Deploy

```sh
cd infra/aws/dev
terraform init
terraform plan
terraform apply
```

## Verify

After `terraform apply`, copy sample artifacts to S3:

```sh
BUCKET="$(terraform output -raw artifacts_bucket_name)"
QUEUE_URL="$(terraform output -raw submissions_queue_url)"

printf '%s\n' 'print(input())' | aws s3 cp - "s3://$BUCKET/submissions/sub_aws_001/main.py"
printf '%s\n' '[{"input":"hello\n","expectedOutput":"hello\n"}]' | aws s3 cp - "s3://$BUCKET/problems/prob_001/testcases.json"
```

Send a judge job:

```sh
aws sqs send-message \
  --queue-url "$QUEUE_URL" \
  --message-body '{
    "submissionId": "sub_aws_001",
    "language": "python",
    "sourceCodeKey": "submissions/sub_aws_001/main.py",
    "testCasesKey": "problems/prob_001/testcases.json",
    "timeLimitMs": 1000
  }'
```

Open a Session Manager shell and inspect logs:

```sh
aws ssm start-session --target "$(terraform output -raw judge_worker_instance_id)"
sudo journalctl -u judge-worker -f
```

Success means the worker log prints a `WorkerResult` JSON with `verdict: "AC"` and the SQS message is deleted.
