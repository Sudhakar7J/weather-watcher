output "api_url" {
  description = "Public base URL for the deployed API."
  value       = aws_lambda_function_url.api.function_url
}

output "health_url" {
  description = "Liveness endpoint, for wiring up an uptime monitor."
  value       = "${aws_lambda_function_url.api.function_url}${var.health_check_path}"
}

output "ecr_repository_url" {
  description = "Push target for CI."
  value       = aws_ecr_repository.app.repository_url
}

output "ecr_push_role_arn" {
  description = "Role CI assumes to push images."
  value       = aws_iam_role.ecr_push.arn
}

output "lambda_function_name" {
  description = "Function name, for invoking it directly in a test."
  value       = aws_lambda_function.api.function_name
}

output "log_group_name" {
  description = "Where runtime logs land. Needed to debug a failed deploy."
  value       = aws_cloudwatch_log_group.lambda.name
}

output "frontend_url" {
  description = "Deployed frontend origin, if configured."
  value       = var.frontend_url
}

output "destroy_instructions" {
  description = "Reminder carried into state so the teardown step is not missed."
  value       = "terraform destroy -auto-approve"
}