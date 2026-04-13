variable "aws_region" {
  type        = string
  description = "AWS region (e.g. us-west-2)"
  default     = "us-west-2"
}

variable "project" {
  type        = string
  description = "Short name prefix for resources"
  default     = "album-store"
}

variable "desired_count" {
  type        = number
  description = "Number of Fargate tasks (more tasks spread photo/list load). Default 1 avoids Fargate vCPU account limits (4 tasks × 4 vCPU/task exceeds many defaults)."
  default     = 2
}
