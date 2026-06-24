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

variable "force_destroy_artifacts_bucket" {
  description = "Whether Terraform may delete the artifacts bucket even when it contains objects. Useful for early experiments."
  type        = bool
  default     = true
}
