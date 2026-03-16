variable "project_name" {
  type = string
}

variable "service_name" {
  type = string
}

variable "cluster_name" {
  type = string
}

variable "cpu" {
  type = number
}

variable "memory" {
  type = number
}

variable "desired_count" {
  type = number
}

variable "task_execution_role_arn" {
  type = string
}

variable "task_role_arn" {
  type = string
}

variable "image" {
  type = string
}

variable "container_name" {
  type = string
}

variable "container_port" {
  type = number
}

variable "subnet_ids" {
  type = list(string)
}

variable "security_group_ids" {
  type = list(string)
}

variable "assign_public_ip" {
  type = bool
}

variable "log_group_name" {
  type = string
}

variable "aws_region" {
  type = string
}

variable "environment" {
  type = list(object({
    name  = string
    value = string
  }))
  default = []
}

variable "target_group_arn" {
  type    = string
  default = null
}

variable "tags" {
  type    = map(string)
  default = {}
}