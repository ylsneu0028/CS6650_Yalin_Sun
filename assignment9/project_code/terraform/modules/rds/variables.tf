variable "name_prefix" {
  type        = string
  description = "Prefix for RDS identifiers (lowercase alnum and hyphens)"
}

variable "vpc_id" {
  type        = string
  description = "VPC for RDS security group and subnet group"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets for the DB subnet group (need 2+ AZs for RDS)"
}

variable "allowed_security_group_ids" {
  type        = list(string)
  description = "Security groups that may connect to MySQL (e.g. ECS tasks)"
}

variable "database_name" {
  type        = string
  description = "Initial database name"
}

variable "master_username" {
  type        = string
  description = "Master username for MySQL"
}
