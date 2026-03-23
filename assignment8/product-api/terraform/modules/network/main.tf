# Fetch the default VPC
data "aws_vpc" "default" {
  default = true
}

locals {
  vpc_id = var.vpc_id != "" ? var.vpc_id : data.aws_vpc.default.id
}

data "aws_vpc" "selected" {
  id = local.vpc_id
}

data "aws_availability_zones" "available" {
  state = "available"
}

# List all subnets in that VPC (legacy / avoid for ECS — includes private RDS subnets)
data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [local.vpc_id]
  }
}

# Public subnets only — Fargate tasks need these for inbound internet + assign_public_ip.
data "aws_subnets" "public" {
  filter {
    name   = "vpc-id"
    values = [local.vpc_id]
  }
  filter {
    name   = "map-public-ip-on-launch"
    values = ["true"]
  }
}

# Create two private subnets for RDS (no public IP auto-assignment, no IGW route).
resource "aws_subnet" "private" {
  count = 2

  vpc_id                  = local.vpc_id
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  cidr_block              = cidrsubnet(data.aws_vpc.selected.cidr_block, 8, 240 + count.index)
  map_public_ip_on_launch = false

  tags = {
    Name = "${var.service_name}-private-${count.index + 1}"
  }
}

resource "aws_route_table" "private" {
  vpc_id = local.vpc_id

  tags = {
    Name = "${var.service_name}-private-rt"
  }
}

resource "aws_route_table_association" "private" {
  count = 2

  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

# Create a security group to allow HTTP to your container port
resource "aws_security_group" "this" {
  name        = "${var.service_name}-sg"
  description = "Allow inbound on ${var.container_port}"
  vpc_id      = local.vpc_id

  ingress {
    from_port   = var.container_port
    to_port     = var.container_port
    protocol    = "tcp"
    cidr_blocks = var.cidr_blocks
    description = "Allow HTTP traffic"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }
}
