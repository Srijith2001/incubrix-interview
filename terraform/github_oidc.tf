# Role the GitHub Actions deploy workflow assumes. GitHub presents a short-lived
# OIDC token and AWS trades it for credentials, so the repository holds a role
# ARN instead of a long-lived access key.
#
# Set github_repository to enable this; leave it empty and none of it is created.

locals {
  github_oidc_enabled = var.github_repository != ""
  github_oidc_url     = "https://token.actions.githubusercontent.com"

  github_oidc_provider_arn = local.github_oidc_enabled ? (
    var.create_github_oidc_provider
    ? aws_iam_openid_connect_provider.github[0].arn
    : data.aws_iam_openid_connect_provider.github[0].arn
  ) : ""
}

# An account can hold only one provider per URL. If another stack already
# created GitHub's, set create_github_oidc_provider = false to reuse it.
resource "aws_iam_openid_connect_provider" "github" {
  count = local.github_oidc_enabled && var.create_github_oidc_provider ? 1 : 0

  url             = local.github_oidc_url
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]
}

data "aws_iam_openid_connect_provider" "github" {
  count = local.github_oidc_enabled && !var.create_github_oidc_provider ? 1 : 0

  url = local.github_oidc_url
}

data "aws_iam_policy_document" "github_actions_assume" {
  count = local.github_oidc_enabled ? 1 : 0

  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [local.github_oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    # Scoped to named refs of one repository. Without this condition any
    # GitHub repository in the world could assume the role.
    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values   = [for ref in var.github_deploy_refs : "repo:${var.github_repository}:${ref}"]
    }
  }
}

resource "aws_iam_role" "github_actions" {
  count = local.github_oidc_enabled ? 1 : 0

  name               = "${local.name}-github-actions"
  description        = "Assumed by GitHub Actions to push images and deploy ${local.name}"
  assume_role_policy = data.aws_iam_policy_document.github_actions_assume[0].json
}

data "aws_iam_policy_document" "github_actions" {
  count = local.github_oidc_enabled ? 1 : 0

  # GetAuthorizationToken has no resource to scope to; it only mints a token
  # whose usefulness is bounded by the push permissions below.
  statement {
    sid       = "EcrLogin"
    effect    = "Allow"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }

  statement {
    sid    = "EcrPush"
    effect = "Allow"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:BatchGetImage",
      "ecr:CompleteLayerUpload",
      "ecr:GetDownloadUrlForLayer",
      "ecr:InitiateLayerUpload",
      "ecr:PutImage",
      "ecr:UploadLayerPart",
    ]
    resources = [aws_ecr_repository.backend.arn]
  }

  # ListServices is how the workflow resolves the service name to an ARN, and
  # it does not accept a resource other than "*".
  statement {
    sid       = "AppRunnerDiscover"
    effect    = "Allow"
    actions   = ["apprunner:ListServices"]
    resources = ["*"]
  }

  statement {
    sid    = "AppRunnerDeploy"
    effect = "Allow"
    actions = [
      "apprunner:DescribeService",
      "apprunner:ListOperations",
      "apprunner:StartDeployment",
    ]
    resources = [aws_apprunner_service.backend.arn]
  }
}

resource "aws_iam_role_policy" "github_actions" {
  count = local.github_oidc_enabled ? 1 : 0

  name   = "${local.name}-github-actions"
  role   = aws_iam_role.github_actions[0].id
  policy = data.aws_iam_policy_document.github_actions[0].json
}
