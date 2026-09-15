resource "aws_apprunner_auto_scaling_configuration_version" "backend" {
  # Service names are capped at 32 characters and a changed setting forces a new
  # version, so the name stays short and unversioned.
  auto_scaling_configuration_name = substr(local.name, 0, 32)

  min_size        = var.min_instances
  max_size        = var.max_instances
  max_concurrency = var.max_concurrency

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_apprunner_service" "backend" {
  service_name = local.name

  source_configuration {
    # Watch the tag so a `docker push` of the same tag rolls the service.
    auto_deployments_enabled = var.auto_deployments_enabled

    authentication_configuration {
      access_role_arn = aws_iam_role.apprunner_ecr_access.arn
    }

    image_repository {
      image_identifier      = "${aws_ecr_repository.backend.repository_url}:${var.image_tag}"
      image_repository_type = "ECR"

      image_configuration {
        port                          = tostring(var.app_port)
        runtime_environment_variables = local.runtime_environment_variables
      }
    }
  }

  instance_configuration {
    cpu    = var.cpu
    memory = var.memory

    # No instance_role_arn: the service calls no AWS API, only frankfurter.dev.
  }

  # The image carries no shell, so there is no Docker HEALTHCHECK to inherit.
  # App Runner probes over HTTP instead.
  health_check_configuration {
    protocol            = "HTTP"
    path                = var.health_check_path
    interval            = 10
    timeout             = 5
    healthy_threshold   = 1
    unhealthy_threshold = 5
  }

  network_configuration {
    ingress_configuration {
      is_publicly_accessible = true
    }
  }

  auto_scaling_configuration_arn = aws_apprunner_auto_scaling_configuration_version.backend.arn

  # The service cannot start until an image exists under this tag; the ECR
  # push happens between the two applies described in the README.
  depends_on = [aws_iam_role_policy_attachment.apprunner_ecr_access]
}
