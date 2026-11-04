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

output "api_instance_id" {
  value = aws_instance.api.id
}

output "api_session_manager_command" {
  value = "aws ssm start-session --target ${aws_instance.api.id}"
}

output "api_port_forward_command" {
  value = "aws ssm start-session --target ${aws_instance.api.id} --document-name AWS-StartPortForwardingSession --parameters '{\"portNumber\":[\"8080\"],\"localPortNumber\":[\"8080\"]}'"
}

output "api_public_url" {
  description = "Public HTTPS API URL when api_domain_name is configured."
  value       = local.public_api_enabled ? "https://${var.api_domain_name}" : null
}

output "api_alb_dns_name" {
  description = "ALB DNS name to configure as the Cloudflare CNAME target."
  value       = local.public_api_enabled ? aws_lb.api[0].dns_name : null
}

output "api_log_group_name" {
  value = aws_cloudwatch_log_group.api.name
}

output "worker_log_group_name" {
  value = aws_cloudwatch_log_group.worker.name
}

output "submissions_dlq_url" {
  value = aws_sqs_queue.submissions_dlq.url
}

output "dlq_redrive_command" {
  description = "Review DLQ messages first, then run this command to move all available messages back to the source queue."
  value       = "aws sqs start-message-move-task --source-arn ${aws_sqs_queue.submissions_dlq.arn} --destination-arn ${aws_sqs_queue.submissions.arn}"
}
