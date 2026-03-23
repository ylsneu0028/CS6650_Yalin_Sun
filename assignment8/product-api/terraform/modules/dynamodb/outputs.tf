output "table_name" {
  description = "DynamoDB table name for shopping carts"
  value       = aws_dynamodb_table.shopping_carts.name
}

output "table_arn" {
  description = "DynamoDB table ARN (for IAM)"
  value       = aws_dynamodb_table.shopping_carts.arn
}
