output "ecs_cluster_name" {
  description = "Name of the created ECS cluster"
  value       = module.ecs.cluster_name
}

output "ecs_service_name" {
  description = "Name of the running ECS service"
  value       = module.ecs.service_name
}

output "rds_address" {
  description = "RDS hostname (private in VPC)"
  value       = module.rds.address
}

output "rds_port" {
  description = "MySQL port"
  value       = module.rds.port
}