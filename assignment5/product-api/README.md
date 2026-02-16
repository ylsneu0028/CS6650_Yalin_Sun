# Product API (Homework 5 - CS6650)

This project implements the **Product API** part of the provided OpenAPI specification.

The system is deployed on AWS ECS (Fargate) with:

- Dockerized Go-based Product API

- Amazon ECR for container images

- Terraform for fully automated infrastructure provisioning

- CloudWatch for centralized logging

With a few Terraform commands, the entire system (ECR, ECS cluster, service, networking) can be deployed on any machine with proper AWS credentials.

## Implemented Endpoints

### GET /products/{productId}

Retrieve a product by its ID.

Responses：
  - 200: returns Product JSON when found
  - 404: returns Error JSON when not found
  - 500 Internal Server Error

### POST /products/{productId}/details

Add or update product details.

Responses：
  - 204: product details updated successfully (no response body)
  - 400: invalid input JSON / schema validation error
  - 404: product not found
  - 500 Internal Server Error

## Authentication

All requests must include the following header:
  - ‘X-API-Key: test’

Requests without this header will be rejected.

## Deploying the System on a New Machine

### Prerequisites

Ensure the following tools are installed:

  - Docker Desktop

  - Terraform ≥ 1.6

  - AWS CLI

  - An active AWS Learner Lab / AWS account with permissions for:

    - ECS

    - ECR

    - EC2 (VPC, Security Groups)

    - AM

    - CloudWatch Logs

### Step 1: Configure AWS Credentials

Export AWS credentials (recommended for Learner Lab):
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_SESSION_TOKEN=...
export AWS_DEFAULT_REGION=us-west-2

Verify access:
aws sts get-caller-identity

### Step 2: Initialize Terraform

cd terraform
terraform init

### Step 3: Deploy Infrastructure and Application

terraform apply -auto-approve

This command will automatically:

  1. Create an ECR repository

  2. Build the Product API Docker image

  3. Push the image to ECR

  4. Create ECS cluster and service

  5. Launch the Product API on AWS Fargate

### Step 4: Verify Service Status

In the AWS Console:

  - ECS → Clusters → yalin-product-api-cluster

  - Confirm the service shows 1/1 tasks running

Logs can be found in:
  CloudWatch → Log groups → /ecs/yalin-product-api

### Step 5: Obtain the Public IP

After the task is running, retrieve the public IP of the ECS task:

aws ec2 describe-network-interfaces \
  --network-interface-ids $(aws ecs describe-tasks \
    --cluster yalin-product-api-cluster \
    --tasks $(aws ecs list-tasks \
      --cluster yalin-product-api-cluster \
      --service-name yalin-product-api \
      --query 'taskArns[0]' --output text) \
    --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
    --output text) \
  --query 'NetworkInterfaces[0].Association.PublicIp' \
  --output text

## Example API Requests (curl)

Assume the public IP is PUBLIC_IP.

### 200 OK – Product Found

curl -i -H "X-API-Key: test" http://PUBLIC_IP:8080/products/1

response example:

{
  "product_id": 1,
  "sku": "ABC-123-XYZ",
  "manufacturer": "Acme Corporation",
  "category_id": 456,
  "weight": 1250,
  "some_other_id": 789
}

### 404 Not Found – Product Does Not Exist

curl -i -H "X-API-Key: test" http://PUBLIC_IP:8080/products/999

Response example:

{
  "error": "NOT_FOUND",
  "message": "Product not found",
  "details": "No product exists with the given productId"
}

### 204 No Content – Add Product Details

curl -i -X POST \
  -H "X-API-Key: test" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 1,
    "sku": "ABC-123",
    "manufacturer": "Acme",
    "category_id": 10,
    "weight": 1000,
    "some_other_id": 42
  }' \
  http://PUBLIC_IP:8080/products/1/details

response: (no body, just 204 status)

### 400 Bad Request – Invalid Input
curl -i -X POST \
  -H "X-API-Key: test" \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://PUBLIC_IP:8080/products/1/details

response example:
{
  "error": "INVALID_INPUT",
  "message": "The provided input data is invalid",
  "details": "product_id must be a positive integer"
}

### 404 Not Found
curl -i -H "X-API-Key: test" http://PUBLIC_IP:8080/products/999/details

response example:
{
  "error": "NOT_FOUND",
  "message": "Product not found",
  "details": "No product exists with the given productId"
}

## Cleanup

To avoid unnecessary AWS charges:
terraform destroy -auto-approve







