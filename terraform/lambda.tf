# Lambda, not App Runner.
#
# App Runner moved to maintenance on 30 April 2026 and no longer accepts new
# customers, so it is not a dependency this project can take. Lambda is also
# the cheaper fit: its always-free tier covers 1M requests and 400k GB-seconds
# a month, and this workload issues a handful of calls a day.
resource "aws_lambda_function" "api" {
  function_name = "${var.project_name}-${var.environment}-api"

  # Reuses the same image CI already builds and pushes, so switching runtimes
  # did not require touching the pipeline.
  package_type = "Image"
  image_uri    = "${aws_ecr_repository.app.repository_url}:${var.image_tag}"
  role         = aws_iam_role.lambda_exec.arn

  architectures = ["arm64"]

  # Generous relative to what this handler does: a burst of cache misses fans
  # out to Open-Meteo, but each call is sub-second.
  memory_size = 512
  timeout     = 30

  # No reserved_concurrent_executions here: setting a cap reduces the
  # account-wide unreserved pool, and Lambda rejects any value that would push
  # it below 10. Leaving it unset avoids an account-level constraint and the
  # free tier already caps total concurrency well above what this needs.

  environment {
    variables = {
      CACHE_TTL       = var.cache_ttl
      MAX_CITIES      = tostring(var.max_cities)
      ALLOWED_ORIGINS = var.allowed_origins
    }
  }

  # Tracing is opt-in and off by default so a failed cold start leaves a
  # CloudWatch log group behind rather than an X-Ray trace nobody reads.
  tracing_config {
    mode = "PassThrough"
  }

  depends_on = [
    aws_cloudwatch_log_group.lambda,
    aws_ecr_repository_policy.app,
  ]
}

# Pre-created so log retention is enforced from the first invocation. Letting
# Lambda create it means retention is "never" unless someone remembers.
resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${var.project_name}-${var.environment}-api"
  retention_in_days = 14
}

resource "aws_iam_role" "lambda_exec" {
  name = "${var.project_name}-${var.environment}-lambda-exec"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

# Least privilege by design: pull this repository's image, write logs. Nothing
# else, and notably no wildcard on other repositories.
data "aws_iam_policy_document" "lambda_logs" {
  statement {
    sid     = "WriteOwnLogs"
    effect  = "Allow"
    actions = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = [
      aws_cloudwatch_log_group.lambda.arn,
      "${aws_cloudwatch_log_group.lambda.arn}:*",
    ]
  }
}

resource "aws_iam_role_policy" "lambda_logs" {
  name   = "logs"
  role   = aws_iam_role.lambda_exec.id
  policy = data.aws_iam_policy_document.lambda_logs.json
}

data "aws_iam_policy_document" "lambda_ecr" {
  statement {
    sid    = "PullOwnImage"
    effect = "Allow"
    actions = [
      "ecr:BatchGetImage",
      "ecr:GetDownloadUrlForLayer",
    ]
    resources = [aws_ecr_repository.app.arn]
  }
}

resource "aws_iam_role_policy" "lambda_ecr" {
  name   = "ecr-pull"
  role   = aws_iam_role.lambda_exec.id
  policy = data.aws_iam_policy_document.lambda_ecr.json
}

resource "aws_iam_role_policy" "lambda_vpc" {
  # Lambda requires the execution role to carry ec2:CreateNetworkInterface and
  # friends before it will accept the ENI management permissions a function
  # needs, even when the function is not attached to a VPC. The actions are
  # never exercised here.
  name   = "vpc-eni-management"
  role   = aws_iam_role.lambda_exec.id
  policy = data.aws_iam_policy_document.lambda_vpc.json
}

data "aws_iam_policy_document" "lambda_vpc" {
  statement {
    sid    = "EniManagement"
    effect = "Allow"
    actions = [
      "ec2:CreateNetworkInterface",
      "ec2:DescribeNetworkInterfaces",
      "ec2:DeleteNetworkInterface",
    ]
    resources = ["*"]
  }
}

# The public entrypoint. Auth type NONE because the API is deliberately
# unauthenticated and CORS-restricted instead; a JWT authoriser would add a
# dependency the product does not need.
resource "aws_lambda_function_url" "api" {
  function_name = aws_lambda_function.api.function_name

  authorization_type = "NONE"

  cors {
    # allow_methods is a set of individual methods, and the API caps each member
    # at 6 characters. "OPTIONS" is 7, so it cannot be listed here anyway;
    # Lambda answers preflight itself whenever a CORS block is present, so
    # omitting it loses nothing.
    allow_methods = ["GET"]
    allow_headers = ["content-type"]
    max_age       = 3000

    # A wildcard origin and credentials cannot be combined, and the app allows
    # every origin by default, so credentials stay off.
    allow_credentials = false

    # Only meaningful when allow_origins is "*"; leaving it out avoids sending
    # a header back that some browsers reject.
    allow_origins = split(",", var.allowed_origins) == ["*"] ? [] : split(",", var.allowed_origins)
  }
}