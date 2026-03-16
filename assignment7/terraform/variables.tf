variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-west-2"
}

variable "project_name" {
  description = "Project prefix"
  type        = string
  default     = "assignment7-ecommerce"
}

# Vocareum used a pre-created role named "LabRole". In your own account (e.g. SSO),
# use an ECS task role that can: ECR pull, CloudWatch Logs, SNS Publish (receiver),
# SQS Receive/Delete (processor). Set this to the role NAME in IAM.
variable "ecs_iam_role_name" {
  description = "Existing IAM role name for ECS task + execution (same as old LabRole usage)"
  type        = string
  default     = "LabRole"
}

# Lambda needs a separate execution role in many accounts; build lambda-processor/deployment.zip first.
variable "enable_lambda_processor" {
  description = "Set true only if deployment.zip exists and ecs_iam_role_name can run Lambda"
  type        = bool
  default     = false
}

variable "receiver_container_port" {
  description = "Receiver app container port"
  type        = number
  default     = 8080
}

variable "receiver_cpu" {
  type    = number
  default = 256
}

variable "receiver_memory" {
  type    = number
  default = 512
}

variable "processor_cpu" {
  type    = number
  default = 256
}

variable "processor_memory" {
  type    = number
  default = 512
}

variable "receiver_desired_count" {
  type    = number
  default = 1
}

variable "processor_desired_count" {
  type    = number
  default = 1
}

variable "processor_worker_count" {
  description = "Worker goroutines in processor container"
  type        = number
  default     = 1
}

variable "log_retention_days" {
  type    = number
  default = 7
}

variable "receiver_source_dir" {
  description = "Path to receiver Docker build context (default: relative to this module)"
  type        = string
  default     = null
}

variable "processor_source_dir" {
  description = "Path to processor Docker build context (default: relative to this module)"
  type        = string
  default     = null
}

variable "receiver_image_tag" {
  type    = string
  default = "latest"
}

variable "processor_image_tag" {
  type    = string
  default = "latest"
}
