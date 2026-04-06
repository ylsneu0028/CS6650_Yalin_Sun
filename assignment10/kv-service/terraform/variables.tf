variable "aws_region" {
  type    = string
  default = "us-west-2"
}

variable "vpc_id" {
  type        = string
  default     = ""
  description = "Leave empty to use the default VPC; set when no default VPC exists"
}

variable "ecs_iam_role_name" {
  type        = string
  default     = "LabRole"
  description = "IAM role name when ecs_iam_role_arn is empty"
}

variable "ecs_iam_role_arn" {
  type        = string
  default     = ""
  description = "Optional full ARN for ECS task/execution role"
}

variable "create_ecs_iam_role" {
  type    = bool
  default = true
  description = <<-EOT
    When true (default), Terraform creates an IAM role with AmazonECSTaskExecutionRolePolicy for ECS pull/logs.
    Set to false on AWS Learner Lab and use the pre-created LabRole via ecs_iam_role_name (default "LabRole").
  EOT
}

variable "ecr_repository_name" {
  type    = string
  default = "yalin-kv-service"
}

variable "service_name" {
  type    = string
  default = "yalin-kv-service"
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "ecs_count" {
  type    = number
  default = 1
}

variable "log_retention_days" {
  type    = number
  default = 7
}

# KV_ROLE: standalone | leader | follower | leaderless
variable "kv_role" {
  type        = string
  default     = "standalone"
  description = "KV_ROLE passed to the container"

  validation {
    condition     = contains(["standalone", "leader", "follower", "leaderless"], var.kv_role)
    error_message = "kv_role must be standalone, leader, follower, or leaderless."
  }
}

# Comma-separated follower base URLs (no trailing paths). Used when kv_role=leader.
variable "kv_peer_urls" {
  type        = string
  default     = ""
  description = "KV_PEER_URLS for the leader (empty for standalone/follower-only tasks)"
}

variable "kv_n" {
  type        = number
  default     = 1
  description = "KV_N (use 5 for Leader–Follower cluster)"
}

variable "kv_r" {
  type        = number
  default     = 1
  description = "KV_R read quorum / strategy (1, 3, or 5)"
}

variable "kv_w" {
  type        = number
  default     = 1
  description = "KV_W write quorum / strategy (1, 3, or 5)"
}

variable "kv_all_urls" {
  type        = string
  default     = ""
  description = "KV_ALL_URLS comma-separated list of all replica base URLs (R>1)"
}

variable "kv_leader_url" {
  type        = string
  default     = ""
  description = "KV_LEADER_URL for followers (R=1 client reads)"
}

variable "kv_self_url" {
  type        = string
  default     = ""
  description = "This task's public base URL (must match one entry in KV_ALL_URLS when using quorum reads)"
}
