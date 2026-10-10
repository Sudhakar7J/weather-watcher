variable "region" {
  description = "AWS region for all resources. Defaults to the region already configured in ~/.aws/config."
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name, used in resource names and tags."
  type        = string
  default     = "dev"
}

variable "project_name" {
  description = "Name prefix for the application resources."
  type        = string
  default     = "weather-watcher"
}

variable "container_port" {
  description = "Port the Go server listens on. Must match the Dockerfile target."
  type        = number
  default     = 8080
}

variable "image_tag" {
  description = "Tag of the ECR image to deploy. Bumped by CI on each build."
  type        = string
  default     = "latest"
}

variable "allowed_origins" {
  description = <<-EOT
    Comma-separated origins permitted by the API's CORS layer, e.g.
    "https://app.example.com,http://localhost:4173". Every request without
    an Origin header is accepted regardless; CORS only constrains browsers.
  EOT
  type        = string
  default     = "*"
}

variable "cache_ttl" {
  description = "How long a geocoded city stays fresh before refetching."
  type        = string
  default     = "1h"
}

variable "max_cities" {
  description = "Maximum cities accepted in one request. The frontend caps to the same value."
  type        = number
  default     = 20
}

variable "health_check_path" {
  description = "Route used by App Runner to decide the container is serving."
  type        = string
  default     = "/api/health"
}

variable "frontend_url" {
  description = "Deployed frontend origin. Recorded in outputs for the README."
  type        = string
  default     = ""
}