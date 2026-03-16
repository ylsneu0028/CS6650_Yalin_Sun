terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

data "aws_caller_identity" "current" {}

data "aws_ecr_authorization_token" "token" {}

provider "docker" {
  registry_auth {
    # Docker 需要纯主机名；proxy_endpoint 可能是 "https://host" 或个别环境返回 "https:"，都需去掉协议头
    address  = trimspace(replace(replace(data.aws_ecr_authorization_token.token.proxy_endpoint, "https://", ""), "https:", ""))
    username = data.aws_ecr_authorization_token.token.user_name
    password = data.aws_ecr_authorization_token.token.password
  }
}