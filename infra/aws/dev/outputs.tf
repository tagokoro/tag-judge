output "account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "aws_region" {
  value = var.aws_region
}

output "vpc_id" {
  value = data.aws_vpc.default.id
}

output "subnet_id" {
  value = local.subnet_id
}

output "artifacts_bucket_name" {
  value = aws_s3_bucket.artifacts.bucket
}

output "submissions_queue_name" {
  value = aws_sqs_queue.submissions.name
}

output "submissions_queue_url" {
  value = aws_sqs_queue.submissions.url
}

output "judge_worker_instance_id" {
  value = aws_instance.judge_worker.id
}

output "session_manager_command" {
  value = "aws ssm start-session --target ${aws_instance.judge_worker.id}"
}

output "worker_logs_command" {
  value = "sudo journalctl -u judge-worker -f"
}
