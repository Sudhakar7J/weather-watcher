# Private registry. The image is built and pushed by CI, and App Runner pulls
# through an access role; nothing is exposed to the anonymous pull path.
resource "aws_ecr_repository" "app" {
  name                 = "${var.project_name}-${var.environment}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

# CI has to authenticate to push, and the token it receives is scoped to this
# repository rather than to the account's whole registry.
resource "aws_ecr_repository_policy" "app" {
  repository = aws_ecr_repository.app.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowPushFromRepository"
        Effect = "Allow"
        Principal = {
          AWS = aws_iam_role.ecr_push.arn
        }
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:CompleteLayerUpload",
          "ecr:InitiateLayerUpload",
          "ecr:PutImage",
        ]
      }
    ]
  })
}

resource "aws_iam_role" "ecr_push" {
  name = "${var.project_name}-${var.environment}-ecr-push"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ecr.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

# Only the three pull actions App Runner actually issues. Notably no
# ecr:* wildcard and no other repository access.
resource "aws_iam_role_policy" "ecr_pull" {
  name = "ecr-pull"
  role = aws_iam_role.ecr_pull.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ecr:BatchGetImage", "ecr:GetDownloadUrlForLayer", "ecr:GetAuthorizationToken"]
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_role" "ecr_pull" {
  name = "${var.project_name}-${var.environment}-ecr-pull"

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