resource "aws_cloudwatch_log_group" "api" {
  name              = "/${var.project_name}/${var.environment}/api"
  retention_in_days = var.cloudwatch_log_retention_days

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/${var.project_name}/${var.environment}/worker"
  retention_in_days = var.cloudwatch_log_retention_days

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}

resource "aws_cloudwatch_metric_alarm" "submissions_oldest_message" {
  alarm_name          = "${local.name_prefix}-submissions-oldest-message"
  alarm_description   = "Submission jobs have not been consumed for five minutes."
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateAgeOfOldestMessage"
  dimensions          = { QueueName = aws_sqs_queue.submissions.name }
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 5
  threshold           = 300
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions
  ok_actions          = local.alarm_actions
}

resource "aws_cloudwatch_metric_alarm" "submissions_dlq_not_empty" {
  alarm_name          = "${local.name_prefix}-submissions-dlq-not-empty"
  alarm_description   = "At least one submission job is in the dead-letter queue."
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateNumberOfMessagesVisible"
  dimensions          = { QueueName = aws_sqs_queue.submissions_dlq.name }
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions
  ok_actions          = local.alarm_actions
}

resource "aws_cloudwatch_metric_alarm" "api_status_check" {
  alarm_name          = "${local.name_prefix}-api-status-check"
  alarm_description   = "The API EC2 instance failed a status check."
  namespace           = "AWS/EC2"
  metric_name         = "StatusCheckFailed"
  dimensions          = { InstanceId = aws_instance.api.id }
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 2
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = local.alarm_actions
  ok_actions          = local.alarm_actions
}

resource "aws_cloudwatch_metric_alarm" "worker_status_check" {
  alarm_name          = "${local.name_prefix}-worker-status-check"
  alarm_description   = "The judge worker EC2 instance failed a status check."
  namespace           = "AWS/EC2"
  metric_name         = "StatusCheckFailed"
  dimensions          = { InstanceId = aws_instance.judge_worker.id }
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 2
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = local.alarm_actions
  ok_actions          = local.alarm_actions
}

resource "aws_cloudwatch_metric_alarm" "api_target_5xx" {
  count = local.public_api_enabled ? 1 : 0

  alarm_name        = "${local.name_prefix}-api-target-5xx"
  alarm_description = "The API returned five or more 5xx responses in five minutes."
  namespace         = "AWS/ApplicationELB"
  metric_name       = "HTTPCode_Target_5XX_Count"
  dimensions = {
    LoadBalancer = aws_lb.api[0].arn_suffix
    TargetGroup  = aws_lb_target_group.api[0].arn_suffix
  }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 5
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions
  ok_actions          = local.alarm_actions
}

resource "aws_sqs_queue_redrive_allow_policy" "submissions_dlq" {
  queue_url = aws_sqs_queue.submissions_dlq.id
  redrive_allow_policy = jsonencode({
    redrivePermission = "byQueue"
    sourceQueueArns   = [aws_sqs_queue.submissions.arn]
  })
}
