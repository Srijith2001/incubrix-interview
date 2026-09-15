# terraform

Infrastructure for the Go API in [`../backend`](../backend). Frontend is not
covered here.

```
ECR repository  ──pull──>  App Runner service  ──HTTPS──>  public URL
```

App Runner over ECS Fargate because this service needs nothing Fargate would
add: no VPC, no subnets, no NAT gateway, no load balancer, no target group, no
ACM certificate. App Runner gives a managed HTTPS endpoint, rolling deploys,
and scale-to-`min_instances` out of one resource.

| File | Holds |
|------|-------|
| `versions.tf` | Terraform/provider constraints, provider config, local backend |
| `variables.tf` | Every input, with defaults and validation |
| `main.tf` | Naming and tagging locals, runtime env vars |
| `ecr.tf` | Image repository and its lifecycle policy |
| `iam.tf` | Role App Runner assumes to pull from ECR |
| `apprunner.tf` | Auto-scaling config and the service itself |
| `outputs.tf` | Service URL, ARNs, image URI |

## Prerequisites

- Terraform >= 1.5, AWS CLI v2, Docker
- AWS credentials with rights over ECR, App Runner, and IAM
- A region where App Runner is available (`ap-south-1` by default)

## First deploy

App Runner will not create a service against a tag that has no image, so the
repository is created first, the image is pushed, then the rest is applied.

```sh
cp terraform.tfvars.example terraform.tfvars   # edit as needed
terraform init

# 1. repository only
terraform apply -target=aws_ecr_repository.backend

# 2. build and push the backend image
REPO=$(terraform output -raw ecr_repository_url)
REGION=$(terraform output -raw aws_region 2>/dev/null || echo ap-south-1)
aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "${REPO%%/*}"
docker build -t "$REPO:latest" ../backend
docker push "$REPO:latest"

# 3. everything else
terraform apply
```

On Windows PowerShell, step 2 reads:

```powershell
$REPO = terraform output -raw ecr_repository_url
$REGION = "ap-south-1"
aws ecr get-login-password --region $REGION | docker login --username AWS --password-stdin $REPO.Split("/")[0]
docker build -t "$REPO`:latest" ../backend
docker push "$REPO`:latest"
```

If Docker is running on Apple silicon, build for the platform App Runner runs:
`docker buildx build --platform linux/amd64 ...`. The Dockerfile already reads
`TARGETOS`/`TARGETARCH`.

Then:

```sh
curl "$(terraform output -raw health_check_url)"
# {"status":"ok","time":"...","uptime":"12s"}
```

## Later deploys

`auto_deployments_enabled` is on by default, so pushing the same tag again is
the whole deploy — App Runner picks up the new image and rolls it. No
`terraform apply` needed for a code-only change.

```sh
docker build -t "$REPO:latest" ../backend && docker push "$REPO:latest"
```

Set `auto_deployments_enabled = false` and bump `image_tag` per release if you
would rather deploys be explicit and rollbacks be a tag change.

## Configuration

Everything is a variable; see `variables.tf` for the full set with defaults.

| Variable | Default | Notes |
|----------|---------|-------|
| `aws_region` | `ap-south-1` | Must support App Runner |
| `environment` | `dev` | Goes into every resource name |
| `app_name` | `incubrix-backend` | Goes into every resource name |
| `app_port` | `8080` | Passed to the container as `PORT` |
| `cpu` / `memory` | `256` / `512` | 0.25 vCPU, 0.5 GB |
| `min_instances` | `1` | `0` is not an App Runner option; 1 keeps it warm |
| `cors_allowed_origins` | *(unset)* | Comma-separated; unset keeps the binary's localhost defaults |
| `health_check_path` | `/api/health` | The image has no shell, so the probe is App Runner's, not Docker's |

`PORT` and `CORS_ALLOWED_ORIGINS` are the only two settings the binary reads, and
both are wired through `local.runtime_environment_variables`.

## State

Local, per `versions.tf`. `.tfstate` and `.tfstate.*` are gitignored, along with
`.terraform/` and real `*.tfvars`. `.terraform.lock.hcl` **is** committed, so
provider versions are reproducible. Moving to S3 is a matter of replacing the
`backend "local"` block and running `terraform init -migrate-state`.

## Teardown

```sh
terraform destroy
```

The ECR repository is `force_delete = true`, so images do not block it.

## Cost

Roughly: App Runner bills provisioned memory continuously for the warm instance
plus vCPU only while requests are in flight; ECR bills storage past the free
tier. `min_instances = 1` is what keeps the meter running — there is no
scale-to-zero. `terraform destroy` stops all of it.
