variable "topic_name" {
  type = string
}

variable "queue_name" {
  type = string
}

variable "tags" {
  type    = map(string)
  default = {}
}