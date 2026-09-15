# Name every resource the same way, so one stack per environment can live in
# one account without collisions.
locals {
  name = "${var.app_name}-${var.environment}"

  tags = merge(
    {
      Application = var.app_name
      Environment = var.environment
      ManagedBy   = "terraform"
      Component   = "backend"
    },
    var.tags,
  )

  # Only set CORS_ALLOWED_ORIGINS when it was supplied. Unset, the binary falls
  # back to its localhost dev defaults rather than to an empty allowlist.
  runtime_environment_variables = merge(
    { PORT = tostring(var.app_port) },
    var.cors_allowed_origins == "" ? {} : { CORS_ALLOWED_ORIGINS = var.cors_allowed_origins },
  )
}
