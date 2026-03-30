variable "vpc_id" {
  description = "Existing VPC id; leave empty to use the account default VPC"
  type        = string
  default     = ""
}

variable "service_name" {
  description = "Base name for SG"
  type        = string
}
variable "container_port" {
  description = "Port to expose on the SG"
  type        = number
}
variable "cidr_blocks" {
  description = "Which CIDRs can reach the service"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}
