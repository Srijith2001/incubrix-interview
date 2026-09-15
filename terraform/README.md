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

## CI/CD

[`.github/workflows/deploy.yml`](../.github/workflows/deploy.yml) runs on a push
to `main` that touches `backend/**`: `go vet` and `go test -race`, then a buildx
build of the backend image.

**The AWS half is commented out** — the ECR login and push, and the whole deploy
job. Nothing in the workflow needs an AWS account as it stands; the image is
built and tagged on the runner and discarded, which still fails the run if the
Dockerfile breaks. The disabled steps stay in the file with the exact commands,
so enabling them is uncommenting, not rewriting.

To turn the deploy on:

1. Apply this stack, so the ECR repository and App Runner service exist.
2. Set `github_repository` and apply again, for the deploy role (below).
3. Store the role ARN as the `AWS_DEPLOY_ROLE_ARN` secret.
4. In `deploy.yml`: uncomment the two AWS steps in `build`, set `push: true`,
   restore the `permissions` block with `id-token: write`, switch the `repo=`
   line to the registry-prefixed form, and uncomment the `deploy` job.

Once on, it pushes to this ECR repository tagged with both the commit SHA and
`latest`, then waits out the App Runner rollout and health-checks the live URL.

It authenticates with OIDC, not an access key. Set `github_repository` to turn
the role on:

```hcl
github_repository = "Srijith2001/incubrix-interview"
```

```sh
terraform apply
terraform output -raw github_actions_role_arn
```

Put that ARN in the repository as the secret `AWS_DEPLOY_ROLE_ARN`
(Settings -> Secrets and variables -> Actions), or:

```sh
gh secret set AWS_DEPLOY_ROLE_ARN --body "$(terraform output -raw github_actions_role_arn)"
```

The trust policy only accepts tokens whose subject matches
`repo:<github_repository>:<ref>` for the refs in `github_deploy_refs`
(`refs/heads/main` by default), and the role can do nothing beyond pushing to
this one repository and deploying this one service.

If the account already has a `token.actions.githubusercontent.com` provider —
only one per account is allowed — set `create_github_oidc_provider = false` and
the existing one is looked up instead.

The workflow's `AWS_REGION`, `ECR_REPOSITORY`, and `APP_RUNNER_SERVICE` env
values must match `aws_region` and `${app_name}-${environment}`. Change one and
change the other.

With `auto_deployments_enabled = true`, the ECR push is what triggers the
rollout and the workflow waits it out. With it `false`, the workflow calls
`StartDeployment` itself. Either way it polls until the service is `RUNNING`
and fails the run if it is not.

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
