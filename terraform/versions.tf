terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.40, < 7.0"
    }
  }

  # Local state on purpose: this stack is small and single-operator, and a
  # remote backend would need its own bootstrapped bucket and lock table.
  # Swap in an "s3" block here when more than one person applies it.
  backend "local" {}
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = local.tags
  }
}
