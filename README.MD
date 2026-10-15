# CS6650 Homework 5 - Product API on ECS via Terraform

Forked from [RuidiH/CS6650_2b_demo](https://github.com/RuidiH/CS6650_2b_demo). This repo deploys
the Product API (from Homework 5 Part II) to AWS ECS Fargate using Terraform, fully automated -
no manual console clicking, no manual `docker build`/`push`.

## What's in this repo

- `src/` - the Go application
  - `product-service.go` - Gin server implementing the Product endpoints from `api.yaml`
  - `go.mod`, `go.sum` - Go module dependencies
  - `Dockerfile` - multi-stage build (golang:1.27 build stage, distroless run stage)
- `terraform/` - infrastructure as code
  - `provider.tf` - configures the AWS and Docker providers; fetches an ECR auth token so
    Terraform's Docker provider can log in automatically (no manual `docker login` needed)
  - `variables.tf` - configuration values (region, service name, **container_port = 8080**,
    task count, log retention)
  - `main.tf` - wires together 4 modules (network, ecr, logging, ecs) and defines the
    `docker_image`/`docker_registry_image` resources that build and push the app image
  - `outputs.tf` - prints the ECS cluster name and service name after deployment
  - `modules/network/` - default VPC lookup, subnets, security group (allows inbound on
    container_port from anywhere)
  - `modules/ecr/` - creates the ECR repository
  - `modules/logging/` - creates a CloudWatch log group for the ECS containers
  - `modules/ecs/` - creates the ECS cluster, task definition, and service (Fargate)

## What changed from the starter repo

- Deleted `src/main.go` (the starter album server), replaced with `src/product-service.go`
  (the Homework 5 Product API), along with its own `go.mod`/`go.sum`
- Replaced `src/Dockerfile` with the Product API's own Dockerfile (uses `golang:1.27` to
  match the Go version required by `go.mod`, and a distroless run stage)
- No changes needed to any `.tf` files - the Product API already listens on port 8080,
  matching `container_port`'s default in `variables.tf`

## How to deploy this on your own machine

### 1. Prerequisites
- [Terraform](https://developer.hashicorp.com/terraform/downloads) installed
- [AWS CLI](https://aws.amazon.com/cli/) installed
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) installed and running
  (Terraform's Docker provider needs a running Docker daemon to build the image)

### 2. Configure AWS credentials
Get temporary credentials from your AWS Academy Learner Lab ("AWS Details" page), then:
```bash
aws configure
# enter your Access Key ID, Secret Access Key, region: us-west-2, output format: (blank)

aws configure set aws_session_token <YOUR-TEMP-SESSION-TOKEN>
```

Verify:
```bash
aws sts get-caller-identity
```

### 3. Deploy
```bash
cd terraform
terraform init
terraform apply -auto-approve
```

This single command builds the Docker image from `src/`, pushes it to a new ECR repository,
and creates the VPC security group, CloudWatch log group, ECS cluster, task definition, and
service - all automatically.

### 4. Get the public IP and test it
From the `terraform` folder:
```bash
aws ec2 describe-network-interfaces \
--network-interface-ids $(
    aws ecs describe-tasks \
    --cluster $(terraform output -raw ecs_cluster_name) \
    --tasks $(
        aws ecs list-tasks \
        --cluster $(terraform output -raw ecs_cluster_name) \
        --service-name $(terraform output -raw ecs_service_name) \
        --query 'taskArns[0]' --output text
    ) \
    --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
    --output text
) \
--query 'NetworkInterfaces[0].Association.PublicIp' \
--output text
```

### 5. Send requests to the Product API

**Get a product:**
```bash
curl "http://<PUBLIC-IP>:8080/products/1"
```
```json
{"product_id":1,"sku":"SEED-0001","manufacturer":"Seed Co.","category_id":1,"weight":100,"some_other_id":1}
```

**Add/update product details:**
```bash
curl -i -X POST "http://<PUBLIC-IP>:8080/products/1/details" \
  -H "Content-Type: application/json" \
  -d '{"product_id":1,"sku":"NEW-SKU","manufacturer":"Updated Corp","category_id":5,"weight":500,"some_other_id":10}'
```

See the [Product API's own README](https://github.com/bhca-cs/cs6650/tree/main/hw05/product-service)
for examples of every response code (200, 204, 400, 404).

## Clean Up
```bash
terraform destroy -auto-approve
```