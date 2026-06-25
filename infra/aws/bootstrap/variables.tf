variable "aws_region" {
  description = "AWS region used for bootstrap resources."
  type        = string
  default     = "ap-northeast-1"
}

variable "project_name" {
  description = "Project name used for AWS resource names."
  type        = string
  default     = "tag-judge"
}

variable "environment" {
  description = "Environment name used for bootstrap resource names."
  type        = string
  default     = "dev"
}

variable "github_repository" {
  description = "GitHub repository allowed to assume the Terraform role, in owner/name form."
  type        = string
  default     = "tagokoro/tag-judge"
}

variable "state_bucket_name" {
  description = "Optional explicit Terraform state bucket name. If null, a deterministic account/region scoped name is used."
  type        = string
  default     = null
}

variable "github_oidc_thumbprints" {
  description = "Thumbprints for the GitHub Actions OIDC provider."
  type        = list(string)
  default     = ["6938fd4d98bab03faadb97b34396831e3780aea1"]
}

variable "github_actions_policy_arns" {
  description = "Managed policies attached to the GitHub Actions Terraform role. Start broad for bootstrap, then narrow after the dev stack stabilizes."
  type        = list(string)
  default     = ["arn:aws:iam::aws:policy/AdministratorAccess"]
}
