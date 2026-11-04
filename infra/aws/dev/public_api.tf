data "aws_acm_certificate" "api" {
  count = local.public_api_enabled ? 1 : 0

  domain      = var.api_domain_name
  statuses    = ["ISSUED"]
  types       = ["AMAZON_ISSUED"]
  most_recent = true
}

resource "aws_security_group" "api_alb" {
  count = local.public_api_enabled ? 1 : 0

  name        = "${local.name_prefix}-api-alb-sg"
  description = "Public HTTPS access to the API ALB"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    description = "HTTP redirect"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description     = "API target"
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.api.id]
  }

  tags = {
    Name        = "${local.name_prefix}-api-alb-sg"
    Project     = var.project_name
    Environment = var.environment
  }
}

resource "aws_lb" "api" {
  count = local.public_api_enabled ? 1 : 0

  name                       = "${local.name_prefix}-api"
  internal                   = false
  load_balancer_type         = "application"
  security_groups            = [aws_security_group.api_alb[0].id]
  subnets                    = data.aws_subnets.default.ids
  drop_invalid_header_fields = true

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}

resource "aws_lb_target_group" "api" {
  count = local.public_api_enabled ? 1 : 0

  name        = "${local.name_prefix}-api"
  port        = 8080
  protocol    = "HTTP"
  target_type = "instance"
  vpc_id      = data.aws_vpc.default.id

  health_check {
    enabled             = true
    path                = "/healthz"
    protocol            = "HTTP"
    matcher             = "204"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    interval            = 30
    timeout             = 5
  }

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}

resource "aws_lb_target_group_attachment" "api" {
  count = local.public_api_enabled ? 1 : 0

  target_group_arn = aws_lb_target_group.api[0].arn
  target_id        = aws_instance.api.id
  port             = 8080
}

resource "aws_lb_listener" "api_http" {
  count = local.public_api_enabled ? 1 : 0

  load_balancer_arn = aws_lb.api[0].arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"
    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

resource "aws_lb_listener" "api_https" {
  count = local.public_api_enabled ? 1 : 0

  load_balancer_arn = aws_lb.api[0].arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = data.aws_acm_certificate.api[0].arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api[0].arn
  }
}
