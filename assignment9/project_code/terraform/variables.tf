# Region to deploy into
variable "aws_region" {
  type    = string
  default = "us-west-2"
}

# When your account has no default VPC (or plan says "no matching EC2 VPC found"),
# set this to an existing VPC id, e.g. from: aws ec2 describe-vpcs --query 'Vpcs[*].VpcId' --output text
variable "vpc_id" {
  type        = string
  default     = ""
  description = "Leave empty to use the default VPC; set when no default VPC exists in this region"
}

variable "private_subnet_ids" {
  type        = list(string)
  default     = []
  description = "Optional override for RDS private subnet IDs; leave empty to use network module generated private subnets"

  validation {
    condition     = length(var.private_subnet_ids) == 0 || length(var.private_subnet_ids) >= 2
    error_message = "private_subnet_ids must be empty (auto mode) or include at least 2 subnet IDs."
  }
}

# Learner Lab uses LabRole; personal AWS accounts often need a different role name or a full ARN.
variable "ecs_iam_role_name" {
  type        = string
  default     = "LabRole"
  description = "IAM role name looked up when ecs_iam_role_arn is not set (needs ecs-tasks trust + ECR/logs/RDS as required)"
}

# If non-empty, used for BOTH execution_role and task_role; skips IAM role name lookup (avoids missing LabRole).
variable "ecs_iam_role_arn" {
  type        = string
  default     = ""
  description = "Optional full ARN for ECS task/execution role; leave empty to resolve ecs_iam_role_name via data source"
}

# When true (and ecs_iam_role_arn is empty), Terraform creates an IAM role with AmazonECSTaskExecutionRolePolicy.
# Set to false on Learner Lab if you use the pre-created LabRole via ecs_iam_role_name.
variable "create_ecs_iam_role" {
  type        = bool
  default     = false
  description = "Create ECS execution/task IAM role in this account; use when LabRole does not exist"
}

# ECR & ECS settings
variable "ecr_repository_name" {
  type    = string
  default = "yalin-store-service"
}

variable "service_name" {
  type    = string
  default = "yalin-product-api"
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "ecs_count" {
  type    = number
  default = 1
}

# How long to keep logs
variable "log_retention_days" {
  type    = number
  default = 7
}

variable "db_name" {
  type        = string
  default     = "store_db"
  description = "MySQL database name created on RDS"
}

variable "db_username" {
  type        = string
  default     = "storeuser"
  description = "MySQL master username (avoid reserved names like admin)"
}

# Step II: set to "dynamodb" for NoSQL cart backend (same API as MySQL Step I)
variable "cart_backend" {
  type        = string
  default     = "mysql"
  description = "Shopping cart storage: mysql | dynamodb"

  validation {
    condition     = contains(["mysql", "dynamodb"], var.cart_backend)
    error_message = "cart_backend must be mysql or dynamodb."
  }
}
