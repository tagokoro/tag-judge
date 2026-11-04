data "aws_secretsmanager_secret" "api" {
  name = "${var.project_name}/${var.environment}/api"
}

resource "aws_iam_role" "api" {
  name = "${local.name_prefix}-api-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "api" {
  name = "${local.name_prefix}-api-policy"
  role = aws_iam_role.api.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "WriteArtifacts"
        Effect   = "Allow"
        Action   = ["s3:PutObject"]
        Resource = "${aws_s3_bucket.artifacts.arn}/*"
      },
      {
        Sid      = "DispatchJobs"
        Effect   = "Allow"
        Action   = ["sqs:GetQueueUrl", "sqs:SendMessage"]
        Resource = aws_sqs_queue.submissions.arn
      },
      {
        Sid      = "ReadAPIConfiguration"
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue"]
        Resource = data.aws_secretsmanager_secret.api.arn
      },
      {
        Sid      = "WriteAPILogs"
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:DescribeLogStreams", "logs:PutLogEvents"]
        Resource = "${aws_cloudwatch_log_group.api.arn}:*"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "api_ssm" {
  role       = aws_iam_role.api.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "api" {
  name = "${local.name_prefix}-api-profile"
  role = aws_iam_role.api.name
}

resource "aws_security_group" "api" {
  name        = "${local.name_prefix}-api-sg"
  description = "API access through SSM or the optional ALB only"
  vpc_id      = data.aws_vpc.default.id

  egress {
    description = "Package downloads, AWS APIs, SSM and RDS"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "${local.name_prefix}-api-sg"
    Project     = var.project_name
    Environment = var.environment
  }
}

resource "aws_vpc_security_group_ingress_rule" "api_from_alb" {
  count = local.public_api_enabled ? 1 : 0

  security_group_id            = aws_security_group.api.id
  referenced_security_group_id = aws_security_group.api_alb[0].id
  description                  = "HTTP from the public API ALB"
  from_port                    = 8080
  to_port                      = 8080
  ip_protocol                  = "tcp"
}

resource "aws_instance" "api" {
  depends_on = [
    aws_iam_role_policy.api,
    aws_iam_role_policy_attachment.api_ssm,
    aws_security_group.db,
    aws_db_instance.default,
  ]

  ami                         = data.aws_ami.ubuntu.id
  instance_type               = var.api_instance_type
  subnet_id                   = local.subnet_id
  vpc_security_group_ids      = [aws_security_group.api.id]
  iam_instance_profile        = aws_iam_instance_profile.api.name
  associate_public_ip_address = true

  metadata_options {
    http_tokens = "required"
  }

  user_data = templatefile("${path.module}/api_user_data.sh.tftpl", {
    aws_region         = var.aws_region
    repository_url     = var.repository_url
    repository_ref     = var.repository_ref
    go_version         = var.api_go_version
    queue_name         = aws_sqs_queue.submissions.name
    bucket_name        = aws_s3_bucket.artifacts.bucket
    secret_arn         = data.aws_secretsmanager_secret.api.arn
    dev_header_auth    = var.api_dev_header_auth
    service_env_script = file("${path.module}/scripts/configure_service_env.py")
    cookie_secure      = local.public_api_enabled
    log_group_name     = aws_cloudwatch_log_group.api.name
  })
  user_data_replace_on_change = true

  lifecycle {
    precondition {
      condition     = !local.public_api_requested || local.public_api_enabled
      error_message = "route53_zone_id is required when api_domain_name is set."
    }
    precondition {
      condition     = !local.public_api_requested || !var.api_dev_header_auth
      error_message = "api_dev_header_auth must remain false when api_domain_name exposes the API publicly."
    }
  }

  root_block_device {
    volume_size = 20
    volume_type = "gp3"
    encrypted   = true
  }

  tags = {
    Name        = "${local.name_prefix}-api"
    Project     = var.project_name
    Environment = var.environment
  }
}
