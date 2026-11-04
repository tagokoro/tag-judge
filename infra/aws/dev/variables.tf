variable "aws_region" {
  description = "AWS region to deploy the judge worker proof of concept."
  type        = string
  default     = "ap-northeast-1"
}

variable "project_name" {
  description = "Project name used for AWS resource names."
  type        = string
  default     = "tag-judge"
}

variable "environment" {
  description = "Environment name used for AWS resource names."
  type        = string
  default     = "dev"
}

variable "subnet_id" {
  description = "Optional subnet ID for the worker EC2 instance. If null, the first default subnet is used."
  type        = string
  default     = null
}

variable "instance_type" {
  description = "EC2 instance type for the initial judge worker."
  type        = string
  default     = "t3.small"
}

variable "api_instance_type" {
  description = "x86_64 EC2 instance type for the API server."
  type        = string
  default     = "t3.small"
}

variable "api_go_version" {
  description = "Pinned Go release used to build the API (without the go prefix)."
  type        = string
  default     = "1.27.3"

  validation {
    condition     = can(regex("^[0-9]+\\.[0-9]+\\.[0-9]+$", var.api_go_version))
    error_message = "Use a release version such as 1.27.3."
  }
}

variable "api_dev_header_auth" {
  description = "Enable X-User-ID authentication for SSM-only development verification. Never enable for public access."
  type        = bool
  default     = false
}

variable "api_domain_name" {
  description = "Optional public DNS name for the API. Its issued ACM certificate and Cloudflare DNS records must be prepared separately."
  type        = string
  default     = null
}

variable "cloudwatch_log_retention_days" {
  description = "Retention period for API and worker application logs."
  type        = number
  default     = 14

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653], var.cloudwatch_log_retention_days)
    error_message = "Use a CloudWatch Logs supported retention period."
  }
}

variable "alarm_sns_topic_arn" {
  description = "Optional SNS topic ARN notified by CloudWatch alarms. Alarms are still created when omitted."
  type        = string
  default     = null
}

variable "repository_url" {
  description = "Git repository cloned by EC2 user_data."
  type        = string
  default     = "https://github.com/tagokoro/tag-judge.git"
}

variable "repository_ref" {
  description = "Git ref checked out by EC2 user_data."
  type        = string
  default     = "main"
}

variable "worker_user" {
  description = "Linux user that owns and runs the judge worker service."
  type        = string
  default     = "judge-worker"
}

variable "rust_toolchain" {
  description = "Rust toolchain installed by rustup on the worker instance."
  type        = string
  default     = "stable"
}

variable "isolate_git_ref" {
  description = "Git ref of ioi/isolate built and installed by EC2 user_data."
  type        = string
  default     = "v2.6"
}

variable "force_destroy_artifacts_bucket" {
  description = "Whether Terraform may delete the artifacts bucket even when it contains objects. Useful for early experiments."
  type        = bool
  default     = true
}
