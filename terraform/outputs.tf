output "api_url" {
  description = "Public base URL for the deployed API."
  value       = "https://${aws_apprunner_service.api.service_url}"
}

output "health_url" {
  description = "Liveness endpoint, for wiring up an uptime monitor."
  value       = "https://${aws_apprunner_service.api.service_url}${var.health_check_path}"
}

output "ecr_repository_url" {
  description = "Push target for CI."
  value       = aws_ecr_repository.app.repository_url
}

output "ecr_push_role_arn" {
  description = "Role CI assumes to push images."
  value       = aws_iam_role.ecr_push.arn
}

output "frontend_url" {
  description = "Deployed frontend origin, if configured."
  value       = var.frontend_url
}

output "destroy_instructions" {
  description = "Reminder carried into state so the teardown step is not missed."
  value       = "terraform destroy -auto-approve"
}