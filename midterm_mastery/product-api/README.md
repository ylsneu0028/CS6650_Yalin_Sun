# Crashing and Recovering (Midterm Mastery - CS6650)

This project demonstrates how a distributed system can fail under unstable dependencies, and how resilience patterns can improve system stability.

The experiment compares two deployments:

1. Problem version – no protection against unstable dependencies

2. Fixed version – uses resilience techniques to prevent cascading failures

The system is deployed on AWS ECS (Fargate) using Terraform.

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

### Step 3: Deploy the problem version with flaky downstream:

terraform apply \
  -var="resilience_mode=problem" \
  -var="downstream_mode=flaky"

### Step 4: Obtain the Public IP

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

### Step 5: Verify the Service
#### Health check:
curl http://$PUBLIC_IP:8080/health

#### Search endpoint:
curl -H "X-API-Key: test" \
"http://$PUBLIC_IP:8080/products/search?q=alpha"

#### Debug endpoint:
curl http://$PUBLIC_IP:8080/debug/stats

### Step 6: Run Load Test
#### Start Locust:
locust -f locustfile.py

#### Open browser:
http://localhost:8089

#### Use the ECS IP as host:
http://PUBLIC_IP:8080

### Step 7 Deploy the Fixed Version
terraform apply \
  -var="resilience_mode=fix" \
  -var="downstream_mode=flaky"

### Step 8 Clean Up Resources
terraform destroy







