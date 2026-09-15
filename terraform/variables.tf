variable "aws_region" {
  description = "AWS region to deploy into. Must be a region where App Runner is available."
  type        = string
  default     = "ap-south-1"
}

variable "environment" {
  description = "Deployment environment. Used in resource names and tags."
  type        = string
  default     = "dev"

  validation {
    condition     = can(regex("^[a-z0-9-]{2,12}$", var.environment))
    error_message = "environment must be 2-12 characters of lowercase letters, digits, or hyphens."
  }
}

variable "app_name" {
  description = "Base name for every resource in this stack."
  type        = string
  default     = "incubrix-backend"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,30}$", var.app_name))
    error_message = "app_name must start with a lowercase letter and contain only lowercase letters, digits, or hyphens."
  }
}

variable "app_port" {
  description = "Port the Go API listens on inside the container. Passed through as PORT."
  type        = number
  default     = 8080

  validation {
    condition     = var.app_port > 0 && var.app_port <= 65535
    error_message = "app_port must be between 1 and 65535."
  }
}

variable "image_tag" {
  description = "ECR image tag App Runner runs. The image must already be pushed; see README."
  type        = string
  default     = "latest"
}

variable "cpu" {
  description = "vCPU allocated per instance, in App Runner units (256 = 0.25 vCPU)."
  type        = string
  default     = "256"

  validation {
    condition     = contains(["256", "512", "1024", "2048", "4096"], var.cpu)
    error_message = "cpu must be one of 256, 512, 1024, 2048, 4096."
  }
}

variable "memory" {
  description = "Memory allocated per instance, in MB."
  type        = string
  default     = "512"

  validation {
    condition     = contains(["512", "1024", "2048", "3072", "4096", "6144", "8192", "10240", "12288"], var.memory)
    error_message = "memory must be a value App Runner accepts, e.g. 512, 1024, 2048."
  }
}

variable "min_instances" {
  description = "Instances kept warm. 1 avoids cold starts; App Runner bills provisioned memory for them."
  type        = number
  default     = 1
}

variable "max_instances" {
  description = "Ceiling App Runner scales out to."
  type        = number
  default     = 3
}

variable "max_concurrency" {
  description = "In-flight requests per instance before App Runner scales out."
  type        = number
  default     = 100
}

variable "health_check_path" {
  description = "HTTP path App Runner probes for instance health."
  type        = string
  default     = "/api/health"
}

variable "cors_allowed_origins" {
  description = "Comma-separated CORS allowlist for the API. Empty string leaves the binary's localhost defaults in place."
  type        = string
  default     = ""
}

variable "auto_deployments_enabled" {
  description = "Redeploy automatically when a new image lands on image_tag in ECR."
  type        = bool
  default     = true
}

variable "image_retention_count" {
  description = "Number of images ECR keeps before the lifecycle policy expires the oldest."
  type        = number
  default     = 10
}

variable "tags" {
  description = "Extra tags merged onto every resource."
  type        = map(string)
  default     = {}
}
