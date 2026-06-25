# Terraform CI/CD

## 目的

Terraform workflow は、AWS dev 環境の judge worker 構成を GitHub Actions から検証し、`main` に merge された変更を apply する。

対象は `infra/aws/dev`。

```text
pull_request
  -> terraform fmt
  -> terraform validate
  -> tflint
  -> terraform plan

push to main
  -> terraform fmt
  -> terraform validate
  -> tflint
  -> terraform plan
  -> terraform apply
```

## 前提

CI の `terraform apply` は GitHub runner 上で実行されるため、ローカル state は使わない。  
Terraform state は S3 backend に保存する。

この state bucket は Terraform 管理対象の `tag-judge-dev-artifacts-*` bucket とは別に、事前に作成しておく。  
この repository では `infra/aws/bootstrap` が state bucket と GitHub Actions 用 IAM role を作成する。

```sh
cd infra/aws/bootstrap
terraform init
terraform apply
```

## GitHub Actions variables

Repository variables に以下を設定する。

```text
AWS_ROLE_ARN
TF_STATE_BUCKET
```

例:

```text
AWS_ROLE_ARN=arn:aws:iam::148362482619:role/tag-judge-github-actions-terraform
TF_STATE_BUCKET=tag-judge-dev-terraform-state-148362482619-ap-northeast-1
```

`infra/aws/bootstrap` を apply した後は、次で設定できる。

```sh
gh variable set AWS_ROLE_ARN --body "$(terraform output -raw github_actions_role_arn)"
gh variable set TF_STATE_BUCKET --body "$(terraform output -raw terraform_state_bucket_name)"
```

## AWS IAM role

GitHub Actions は OIDC で AWS role を assume する。  
access key / secret key は GitHub secrets に置かない。

信頼ポリシーでは、この repository の `main` branch と pull request からの実行だけを許可する。

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::148362482619:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": [
            "repo:tagokoro/tag-judge:ref:refs/heads/main",
            "repo:tagokoro/tag-judge:pull_request"
          ]
        }
      }
    }
  ]
}
```

初期検証では、この role に `AdministratorAccess` を付けても動作確認はできる。  
ただしポートフォリオとして説明する場合は、後で `infra/aws/dev` が作成・更新する resource に絞った policy へ縮小する。
