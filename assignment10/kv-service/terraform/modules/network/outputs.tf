output "vpc_id" {
  description = "VPC id used by the service"
  value       = local.vpc_id
}

output "subnet_ids" {
  description = "All subnet IDs in the VPC"
  value       = data.aws_subnets.default.ids
}

output "public_subnet_ids" {
  description = "Public subnets for ECS Fargate"
  value       = data.aws_subnets.public.ids
}

output "security_group_id" {
  description = "Security group ID for ECS"
  value       = aws_security_group.this.id
}

output "private_subnet_ids" {
  description = "Private subnet IDs"
  value       = aws_subnet.private[*].id
}
