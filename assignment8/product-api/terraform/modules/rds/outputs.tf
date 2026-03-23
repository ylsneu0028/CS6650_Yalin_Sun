output "address" {
  description = "RDS hostname for applications"
  value       = aws_db_instance.this.address
}

output "port" {
  description = "MySQL port"
  value       = aws_db_instance.this.port
}

output "database_name" {
  description = "Logical database name"
  value       = aws_db_instance.this.db_name
}

output "master_username" {
  description = "Master username"
  value       = aws_db_instance.this.username
}

output "master_password" {
  description = "Master password (sensitive)"
  value       = random_password.master.result
  sensitive   = true
}
