variable "project_name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "public_subnet_ids" {
  type = list(string)
}

variable "alb_security_group_id" {
  type = string
}

variable "target_port" {
  type = number
}

variable "health_check_path" {
  type = string
}

variable "tags" {
  type    = map(string)
  default = {}
}