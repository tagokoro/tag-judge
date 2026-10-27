terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

data "aws_caller_identity" "current" {}

locals {
  name_prefix       = "${var.project_name}-${var.environment}"
  state_bucket_name = coalesce(var.state_bucket_name, "${var.project_name}-${var.environment}-terraform-state-${data.aws_caller_identity.current.account_id}-${var.aws_region}")
}

resource "aws_s3_bucket" "terraform_state" {
  bucket = local.state_bucket_name

  tags = {
    Name        = "${local.name_prefix}-terraform-state"
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "terraform-bootstrap"
  }
}

resource "aws_s3_bucket_public_access_block" "terraform_state" {
  bucket = aws_s3_bucket.terraform_state.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "terraform_state" {
  bucket = aws_s3_bucket.terraform_state.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "terraform_state" {
  bucket = aws_s3_bucket.terraform_state.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_iam_openid_connect_provider" "github_actions" {
  url = "https://token.actions.githubusercontent.com"

  client_id_list = [
    "sts.amazonaws.com"
  ]

  thumbprint_list = var.github_oidc_thumbprints

  tags = {
    Name        = "${local.name_prefix}-github-actions-oidc"
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "terraform-bootstrap"
  }
}

resource "aws_iam_role" "github_actions_terraform" {
  name = "${local.name_prefix}-github-actions-terraform"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = aws_iam_openid_connect_provider.github_actions.arn
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
          }
          StringLike = {
            "token.actions.githubusercontent.com:sub" = [
              "repo:${var.github_repository}:ref:refs/heads/main",
            ]
          }
        }
      }
    ]
  })

  tags = {
    Name        = "${local.name_prefix}-github-actions-terraform"
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "terraform-bootstrap"
  }
}

resource "aws_iam_role_policy_attachment" "github_actions_terraform" {
  for_each = toset(var.github_actions_policy_arns)

  role       = aws_iam_role.github_actions_terraform.name
  policy_arn = each.value
}
