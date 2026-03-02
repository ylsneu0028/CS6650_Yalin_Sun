variable "aws_region" {
  type    = string
  default = "us-west-2"
}

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

variable "log_retention_days" {
  type    = number
  default = 7
}

# ---------- A7 追加变量 ----------
variable "resilience_mode" {
  type    = string
  default = "fix"
}

variable "downstream_mode" {
  type    = string
  default = "ok"
}
