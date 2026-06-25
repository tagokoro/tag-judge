# AWS bootstrap

This stack creates the AWS resources required before the Terraform GitHub Actions workflow can run.

## Responsibilities

- S3 bucket for Terraform remote state
- GitHub Actions OIDC provider
- IAM role assumed by GitHub Actions

This is intentionally separate from `infra/aws/dev`. The `dev` stack manages the judge worker resources; the `bootstrap` stack makes it possible for GitHub Actions to manage that stack.

## Why this is separate

GitHub Actions cannot run `terraform init` with an S3 backend until the state bucket and IAM role already exist.

```text
local machine
  -> terraform apply infra/aws/bootstrap
  -> creates state bucket and GitHub Actions role

GitHub Actions
  -> assumes that role through OIDC
  -> terraform plan/apply infra/aws/dev
```

## Apply

Run this once from a local machine that is already authenticated with AWS.

```sh
cd infra/aws/bootstrap
terraform init
terraform fmt
terraform validate
terraform plan
terraform apply
```

## Configure GitHub repository variables

After apply, set these GitHub repository variables:

```sh
gh variable set AWS_ROLE_ARN --body "$(terraform output -raw github_actions_role_arn)"
gh variable set TF_STATE_BUCKET --body "$(terraform output -raw terraform_state_bucket_name)"
```

Equivalent values:

```text
AWS_ROLE_ARN=<github_actions_role_arn output>
TF_STATE_BUCKET=<terraform_state_bucket_name output>
```

## Security note

The initial Terraform role attaches `AdministratorAccess` through `github_actions_policy_arns`.

This is acceptable for the first AWS deployment check because the goal is to verify the end-to-end judge worker path quickly:

```text
SQS job -> EC2 judge-worker -> S3 artifacts -> judge-core -> log result
```

After the stack stabilizes, replace `AdministratorAccess` with a narrower policy that only permits the resources managed under `infra/aws/dev`.
