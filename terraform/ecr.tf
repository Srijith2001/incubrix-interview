resource "aws_ecr_repository" "backend" {
  name = local.name

  # Mutable so that pushing the same tag again is a deploy; App Runner's
  # auto-deployment watches the tag, not a digest.
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  # Nothing here is worth keeping across a teardown, and a lingering repository
  # would block a later apply that wants the same name.
  force_delete = true
}

resource "aws_ecr_lifecycle_policy" "backend" {
  repository = aws_ecr_repository.backend.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Expire all but the newest ${var.image_retention_count} images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = var.image_retention_count
        }
        action = { type = "expire" }
      },
    ]
  })
}
