output "alb_dns_name" {
  description = "Public base URL host (use http://<this>)"
  value       = aws_lb.main.dns_name
}

output "base_url" {
  description = "Full HTTP base URL for ChaosArena submit"
  value       = "http://${aws_lb.main.dns_name}"
}

output "ecr_repository_url" {
  description = "docker push target"
  value       = aws_ecr_repository.app.repository_url
}

output "s3_bucket" {
  value = aws_s3_bucket.photos.bucket
}

output "dynamodb_tables" {
  value = {
    albums = aws_dynamodb_table.albums.name
    photos = aws_dynamodb_table.photos.name
  }
}

output "ecs_cluster_name" {
  value = aws_ecs_cluster.main.name
}

output "ecs_service_name" {
  value = aws_ecs_service.app.name
}
