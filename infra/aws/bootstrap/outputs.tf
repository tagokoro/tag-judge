output "account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "aws_region" {
  value = var.aws_region
}

output "terraform_state_bucket_name" {
  value = aws_s3_bucket.terraform_state.bucket
}

output "github_actions_role_arn" {
  value = aws_iam_role.github_actions_terraform.arn
}

output "github_repository" {
  value = var.github_repository
}
