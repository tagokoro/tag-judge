# The Next.js frontend runs on the API instance (port 3000) and shares the
# API ALB: requests for frontend_domain_name go to it by host header, and
# everything else keeps going to the API.

data "aws_acm_certificate" "frontend" {
  count = local.frontend_enabled ? 1 : 0

  domain      = var.frontend_domain_name
  statuses    = ["ISSUED"]
  types       = ["AMAZON_ISSUED"]
  most_recent = true
}

resource "aws_lb_listener_certificate" "frontend" {
  count = local.frontend_enabled ? 1 : 0

  listener_arn    = aws_lb_listener.api_https[0].arn
  certificate_arn = data.aws_acm_certificate.frontend[0].arn
}

resource "aws_lb_target_group" "frontend" {
  count = local.frontend_enabled ? 1 : 0

  name        = "${local.name_prefix}-frontend"
  port        = 3000
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

resource "aws_lb_target_group_attachment" "frontend" {
  count = local.frontend_enabled ? 1 : 0

  target_group_arn = aws_lb_target_group.frontend[0].arn
  target_id        = aws_instance.api.id
  port             = 3000
}

resource "aws_lb_listener_rule" "frontend" {
  count = local.frontend_enabled ? 1 : 0

  listener_arn = aws_lb_listener.api_https[0].arn
  priority     = 100

  condition {
    host_header {
      values = [var.frontend_domain_name]
    }
  }

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.frontend[0].arn
  }
}

resource "aws_vpc_security_group_ingress_rule" "frontend_from_alb" {
  count = local.frontend_enabled ? 1 : 0

  security_group_id            = aws_security_group.api.id
  referenced_security_group_id = aws_security_group.api_alb[0].id
  description                  = "Frontend HTTP from the public ALB"
  from_port                    = 3000
  to_port                      = 3000
  ip_protocol                  = "tcp"
}
