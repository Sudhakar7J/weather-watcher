# Instance-based, not Fargate. Traffic for this workload is a handful of
# people refreshing a weather dashboard, and the always-warm instance is the
# whole appeal; Fargate would trade that away to save cost on idle CPU that an
# instance plan does not charge for anyway.
resource "aws_apprunner_service" "api" {
  service_name = "${var.project_name}-${var.environment}-api"

  source_configuration {
    # New images ship on an explicit deploy rather than whenever a tag moves.
    # Auto-deploy would make an accidental tag overwrite a production rollout.
    auto_deployments_enabled = false

    image_repository {
      image_repository_type = "ECR"
      image_identifier      = aws_ecr_repository.app.arn

      image_configuration {
        port = tostring(var.container_port)

        runtime_environment_variables = {
          PORT            = tostring(var.container_port)
          CACHE_TTL       = var.cache_ttl
          MAX_CITIES      = tostring(var.max_cities)
          ALLOWED_ORIGINS = var.allowed_origins
        }
      }
    }

    authentication_configuration {
      access_role_arn = aws_iam_role.ecr_pull.arn
    }
  }

  instance_configuration {
    cpu               = 1
    memory            = 2048
    instance_role_arn = aws_iam_role.apprunner_instance.arn
  }

  # /api/health answers even when Open-Meteo is unreachable, so an upstream
  # outage cannot cascade into App Runner restarting healthy containers.
  health_check_configuration {
    protocol            = "HTTP"
    path                = var.health_check_path
    interval            = 10
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }

  tags = {
    Name = "${var.project_name}-${var.environment}-api"
  }
}

# The service makes no AWS API calls, so this role intentionally carries no
# permissions at all. App Runner still requires the role to exist.
resource "aws_iam_role" "apprunner_instance" {
  name = "${var.project_name}-${var.environment}-apprunner-instance"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "tasks.apprunner.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}