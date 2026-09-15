output "service_url" {
  description = "Public HTTPS endpoint for the API."
  value       = "https://${aws_apprunner_service.backend.service_url}"
}

output "health_check_url" {
  description = "Health endpoint, handy for a post-deploy curl."
  value       = "https://${aws_apprunner_service.backend.service_url}${var.health_check_path}"
}

output "service_arn" {
  description = "ARN of the App Runner service."
  value       = aws_apprunner_service.backend.arn
}

output "ecr_repository_url" {
  description = "ECR repository to build and push the backend image to."
  value       = aws_ecr_repository.backend.repository_url
}

output "image_uri" {
  description = "Full image reference App Runner runs."
  value       = "${aws_ecr_repository.backend.repository_url}:${var.image_tag}"
}

output "aws_region" {
  description = "Region the stack was applied to. Used by the docker login in the README."
  value       = var.aws_region
}

output "github_actions_role_arn" {
  description = "Role ARN for the AWS_DEPLOY_ROLE_ARN repository secret. Null when github_repository is unset."
  value       = local.github_oidc_enabled ? aws_iam_role.github_actions[0].arn : null
}
