# Private registry. The image is built and pushed by CI, and Lambda pulls it
# through its own execution role rather than anything shared.
resource "aws_ecr_repository" "app" {
  name                 = "${var.project_name}-${var.environment}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

# Lambda authenticates with its execution role and the ECR authorization
# token, which does not need a repository policy. The allow is kept scoped to
# this one repository rather than the account-wide default, so a compromised
# CI token cannot push elsewhere.
resource "aws_ecr_repository_policy" "app" {
  repository = aws_ecr_repository.app.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "DenyUnencryptedUpload"
        Effect    = "Deny"
        Principal = "*"
        Action    = "ecr:PutImage"
        Condition = {
          StringNotEquals = {
            "ecr:EncryptionType" = "AES256"
          }
        }
      }
    ]
  })
}

# CI authenticates to push through this role. The trust policy restricts it to
# ECR, so the principal cannot be used for anything else.
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