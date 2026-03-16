variable "log_group_name" {
  type = string
}

variable "retention_in_days" {
  type = number
}

variable "tags" {
  type    = map(string)
  default = {}
}
