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

### Worker recreation prerequisites

The existing RDS `judge` database must already have the application migrations
and the dedicated `judge_worker` login and grants. Recreating EC2 does not
initialize a new database or reset DB passwords.

Before planning, create a Secrets Manager secret named
`tag-judge/dev/worker-db` (or `<project_name>/<environment>/worker-db`) in the
deployment region. Its JSON value must contain the existing worker DB login:

```json
{"username": "judge_worker", "password": "<existing worker DB password>"}
```

Use the worker password, not the RDS administrator or API password. Do not commit
the secret value or pass it through Terraform variables. Terraform reads only
the secret metadata and grants the EC2 role access to this specific secret.

During EC2 setup, cloud-init installs OpenSSL build dependencies and RDS root
certificates, fetches the secret using the instance role, then atomically creates
`/etc/tag-judge/worker.env` with mode `0600` inside a root-only directory. Only
after successful setup does it start the worker service. The secret value is not
printed to cloud-init logs or embedded in EC2 user data.

Secret changes are not fetched on every service restart. After changing the
worker DB password, update its secret and regenerate the environment file or
recreate EC2. RDS administrator password rotation is independent of this login.

Run the environment-generation tests without AWS access from the repository root:

```sh
python3 -B -m unittest discover -s infra/aws/dev/tests -v
```

### Plan and apply

```sh
cd infra/aws/dev
terraform init
terraform plan
terraform apply
```

## Verify

After `terraform apply`, open a Session Manager shell:

```sh
aws ssm start-session --target "$(terraform output -raw judge_worker_instance_id)"
```

On EC2, check setup completion and service health:

```sh
sudo cloud-init status --wait
sudo systemctl is-active judge-worker
sudo stat -c '%a %U' /etc/tag-judge/worker.env
sudo journalctl -u judge-worker -n 50 --no-pager
```

The service should be active and the environment file should be owned by root
with mode `600`. Inspect file metadata rather than printing its credentials.

For end-to-end verification, create submissions through the API as described in
the [project README](../../../README.md). The API records the submission and
immutable test-case snapshot before dispatching a job. Verify an AC submission
and a WA submission using the submission-detail API, including case results and
scores. This verifies result persistence as well as code execution.
