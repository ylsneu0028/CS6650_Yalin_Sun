output "alb_dns_name" {
  value = module.alb.alb_dns_name
}

output "receiver_url" {
  value = "http://${module.alb.alb_dns_name}"
}

output "sns_topic_arn" {
  value = module.messaging.sns_topic_arn
}

output "sqs_queue_url" {
  value = module.messaging.sqs_queue_url
}

output "sqs_queue_arn" {
  value = module.messaging.sqs_queue_arn
}

output "ecs_cluster_name" {
  value = aws_ecs_cluster.main.name
}

# Part III (only when enable_lambda_processor = true)
output "lambda_order_processor_name" {
  value = var.enable_lambda_processor ? aws_lambda_function.order_processor[0].function_name : null
}