# AWS dev API and judge worker

The API runs on a separate EC2 instance with its own IAM role and an inbound-closed
security group. Access HTTP through SSM port forwarding by default. It shares the
existing RDS, S3 bucket and SQS queue with the worker. An optional public HTTPS
endpoint uses an ALB, an existing ACM certificate, and Cloudflare DNS; the instance
itself is never opened to the internet. The Next.js frontend can run on the same
instance behind the same ALB under its own domain.

## API deployment prerequisites

Before planning, create a Secrets Manager secret named `tag-judge/dev/api`
(or `<project_name>/<environment>/api`) in the deployment region:

```json
{"DATABASE_URL": "postgresql://judge_api:<URL-encoded-password>@<RDS-host>:5432/judge?sslmode=verify-full&sslrootcert=/opt/tag-judge-api/rds-bundle.pem"}
```

The existing `judge` database must have all application migrations and a dedicated
`judge_api` login with application table/sequence privileges, including sessions,
code runs, submission snapshots and the submission job outbox. This stack does not
create DB users, grant privileges, run migrations, or change passwords. Do not use
the administrator or worker credentials. Use the default Secrets Manager encryption
key; a customer-managed key needs a separate scoped `kms:Decrypt` grant.

The shared `scripts/configure_service_env.py` copies the complete URL unchanged;
it does not build URLs, encode passwords, choose a DB login, or add TLS options.
Percent-encode reserved characters in usernames/passwords before saving the URL.
Use the actual RDS endpoint and keep the TLS parameters shown in the examples.
The API CA bundle is installed at `/opt/tag-judge-api/rds-bundle.pem`; the worker
uses the system trust store configured during setup.

When migrating from the previous `{username, password}` format, update both
Secrets **before applying** this change. Old-format secrets are rejected. The
user-data changes can replace both EC2 instances; inspect the plan first. Existing
instances keep their old configuration until explicitly reconfigured or replaced.
Changing the DB endpoint, port, database, or password requires updating the URLs
in the affected Secrets.

The secret may also include `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`,
`GOOGLE_OAUTH_REDIRECT_URL` (supply all three together), and
`AUTH_SUCCESS_REDIRECT_URL`. Without these, Google login is not configured.
For browser login through the tunnel, use the localhost callback URL and register
the same URL with Google. When `api_domain_name` is configured, cookies are marked
Secure. Browsers reach the API only through the frontend's `/api/*` rewrite, so the
session cookie must be set on the frontend domain: with the public frontend, use the
frontend domain, not the API domain, for both URLs:

```json
{
  "GOOGLE_OAUTH_REDIRECT_URL": "https://judge.tagokoro.dev/api/v1/auth/google/callback",
  "AUTH_SUCCESS_REDIRECT_URL": "https://judge.tagokoro.dev/"
}
```

Register that exact callback in Google Cloud. With the MCP server, its OAuth issuer
(`PUBLIC_BASE_URL`) defaults to the origin of `AUTH_SUCCESS_REDIRECT_URL`
(`https://judge.tagokoro.dev`). OAuth settings are
read at instance provisioning time; rotate the secret and rerun the configuration
helper plus `systemctl restart judge-api`, or replace the instance.

For development header authentication, explicitly set `api_dev_header_auth = true`
in a local, uncommitted tfvars file. It defaults to false. Use an existing database
user ID when testing authenticated endpoints. Never enable header authentication
when exposing the API publicly.

Cloud-init downloads a pinned, checksum-verified Go release and builds
`backend/cmd/api` at `repository_ref`. The ref must be available in the remote
repository. The service runs as `judge-api`. Its root-owned environment file is
generated from Secrets Manager with mode `0600`; DB TLS verifies the RDS hostname
and CA bundle when the URL above is used. Secrets are not interpolated into Terraform state or user data.
Secret updates require rerunning configuration and restarting the service, or
recreating the API instance. Configuration helpers remain in `/opt/tag-judge-setup`.

## Optional public HTTPS endpoint

The default remains SSM-only. Before enabling the ALB, request an ACM certificate
for the API domain in the deployment region. Add the DNS validation CNAME shown by
ACM to Cloudflare and wait until the certificate status is `Issued`. Terraform
looks up the newest issued Amazon certificate for the exact domain; it does not
manage Cloudflare or the certificate lifecycle.

Then set the domain in an uncommitted tfvars file:

```hcl
api_domain_name = "judge-api.tagokoro.dev"
```

For the automatic `main` deployment, configure the equivalent optional GitHub
Actions repository variables:

```sh
gh variable set API_DOMAIN_NAME --body "judge-api.tagokoro.dev"
# Optional alarm notifications:
gh variable set ALARM_SNS_TOPIC_ARN --body "arn:aws:sns:ap-northeast-1:123456789012:tag-judge-alerts"
```

After apply, configure the API hostname in Cloudflare as a CNAME whose target is:

```sh
terraform output -raw api_alb_dns_name
```

Cloudflare proxying may be enabled. Use SSL/TLS mode `Full (strict)` so both the
browser-to-Cloudflare and Cloudflare-to-ALB connections are authenticated and
encrypted. Do not use Flexible mode. HTTP redirects to HTTPS, the ALB accepts TLS
1.2/1.3, and EC2 port 8080 accepts traffic only from the ALB security group. Do not
enable `api_dev_header_auth` on a public endpoint. The final URL is available as
`terraform output -raw api_public_url`.

After a separately authorized apply, inspect the API through Session Manager:

```sh
aws ssm start-session --target "$(terraform output -raw api_instance_id)"
```

On the API EC2 instance:

```sh
sudo cloud-init status --wait
sudo systemctl is-active judge-api
sudo stat -c '%a %U' /etc/tag-judge/api.env
sudo journalctl -u judge-api -n 50 --no-pager
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/api/v1/problems
```

`/healthz` returns 204 but does not check the DB. The problems endpoint additionally
exercises database access. For local access, run this on your computer (requires
the AWS CLI Session Manager plugin and permission to start port forwarding):

```sh
aws ssm start-session \
  --target "$(terraform output -raw api_instance_id)" \
  --document-name AWS-StartPortForwardingSession \
  --parameters '{"portNumber":["8080"],"localPortNumber":["8080"]}'
```

Then use `http://localhost:8080`. The instance uses a public IP for
outbound package downloads and AWS API access; its security group has no inbound
rules unless the optional ALB is enabled, in which case only ALB-to-instance traffic
is allowed.

## Optional public frontend

The Next.js frontend runs on the API instance and shares the API ALB, so it needs
the public API endpoint above. Requests whose host is the frontend domain go to the
frontend on port 3000; every other host keeps going to the API. Request an ACM
certificate for the frontend domain in the deployment region the same way, wait
until it is `Issued`, then set the domain next to the API one:

```hcl
api_domain_name      = "judge-api.tagokoro.dev"
frontend_domain_name = "judge.tagokoro.dev"
```

```sh
gh variable set FRONTEND_DOMAIN_NAME --body "judge.tagokoro.dev"
```

Point the frontend hostname at the same ALB with a Cloudflare CNAME (target:
`terraform output -raw api_alb_dns_name`, SSL/TLS mode `Full (strict)`). The URL is
`terraform output -raw frontend_public_url`. Update the API secret's Google OAuth
URLs to the frontend domain as described above.

Cloud-init downloads a pinned, checksum-verified Node.js (`frontend_node_version`),
builds `frontend/` at `repository_ref` with `API_SERVER_ORIGIN=http://127.0.0.1:8080`
(the rewrites to the API are fixed at build time), and runs `.next/standalone` as the
`judge-web` service on port 3000. Its log goes to the API log group under the
`<instance-id>-web` stream. `next build` runs beside the API on a t3.small, so
cloud-init adds a 2 GB swap file first; use a larger `api_instance_type` if builds
are slow or fail. Setting the domain changes the API user data and replaces the API
instance, as does any later change to it; without the domain, nothing changes.

The build runs third-party npm code, so it runs as a separate `judge-build` user on
a copy of `frontend/`, with `npm ci --ignore-scripts`, and with the instance
metadata service blocked (`IPAddressDeny=169.254.169.254`); `judge-web` is blocked
from it too. Neither can read the role's credentials or touch the API binary. The
CloudWatch agent starts before the build, so a failed build still ships the API log;
check `/var/log/cloud-init-output.log` if the frontend target stays unhealthy.

Merging frontend changes to `main` does not redeploy them. The frontend, like the
API, is built only when the instance is created, and the deploy workflow runs only
for `infra/aws/dev/**` changes. To ship new `main`, replace the API instance in a
quiet window (the API is down until the new instance passes its health check):

```sh
terraform apply -replace=aws_instance.api
```

On the API EC2 instance:

```sh
sudo systemctl is-active judge-web
sudo tail -n 50 /var/log/tag-judge/web.log
curl -i http://localhost:3000/healthz
curl -i http://localhost:3000/
```

`/healthz` returns 204 without calling the API; it is the ALB health check.

## AWS submission smoke test

The smoke test exercises API -> RDS/S3/SQS -> Worker -> RDS -> API with a real
Python submission. It creates a problem and test case, waits for AC, and returns
the problem to private visibility. The problem, submission, and artifacts remain as
test evidence because submissions prevent problem deletion. Run it only through the
SSM tunnel with
`api_dev_header_auth = true`; the UUID must identify an existing database user.

```sh
infra/aws/dev/scripts/aws_submission_smoke_test.sh \
  http://localhost:8080 <existing-user-uuid>
```

Keep header authentication disabled for the public ALB. For public-path verification,
log in through Google OAuth and use the session cookie instead. The smoke script is
intentionally not run by CI because it mutates the deployed database and requires a
live worker.

## Logs, alarms, and DLQ recovery

API and worker stdout/stderr are sent to `/tag-judge/dev/api` and
`/tag-judge/dev/worker` with 14-day retention by default. Terraform creates alarms
for EC2 status checks, submission queue age, a non-empty DLQ, and (when enabled)
ALB target 5xx responses. Set `alarm_sns_topic_arn` to an existing SNS topic to
receive alarm and recovery notifications; without it the alarms remain visible in
CloudWatch but do not notify.

Inspect the DLQ before replaying it. After fixing the cause, move available messages
back to the source queue with the generated command:

```sh
terraform output -raw dlq_redrive_command
# Review the command and then execute it.
```

The SQS redrive allow policy restricts replay to the submissions source queue. Watch
the worker log group and queue/DLQ metrics while replay is active.

Review the plan for unrelated replacements: the existing worker uses a
`most_recent` AMI. Merging changes under `infra/aws/dev` into `main` triggers
the repository's automatic Terraform apply workflow.

Local checks for the API environment helper:

```sh
python3 -B -m unittest discover -s infra/aws/dev/scripts -p 'test_*.py'
```

## Existing worker setup

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
deployment region. Its JSON value must contain the complete worker connection URL:

```json
{"JUDGE_DATABASE_URL": "postgresql://judge_worker:<URL-encoded-password>@<RDS-host>:5432/judge?sslmode=require"}
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
