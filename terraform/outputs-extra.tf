output "app_runner_service_name" {
  description = "Service name used by CI to trigger and poll deployments."
  value       = aws_apprunner_service.api.service_name
}

output "app_runner_service_arn" {
  description = "Full ARN of the App Runner service."
  value       = aws_apprunner_service.api.arn
}