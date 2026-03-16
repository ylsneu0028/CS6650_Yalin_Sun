# ------------------------------------------------------------------------------
# Part III: Lambda processor subscribed to SNS (optional)
# Run from assignment7: cd lambda-processor && make build  before terraform apply
# Set enable_lambda_processor = true only when deployment.zip exists and IAM allows Lambda.
# ------------------------------------------------------------------------------

resource "aws_lambda_function" "order_processor" {
  count = var.enable_lambda_processor ? 1 : 0

  filename         = "${path.module}/../lambda-processor/deployment.zip"
  function_name    = "${var.project_name}-order-processor"
  role             = data.aws_iam_role.ecs_task.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  memory_size      = 512
  timeout          = 10
  source_code_hash = filebase64sha256("${path.module}/../lambda-processor/deployment.zip")
  tags             = local.common_tags
}

resource "aws_lambda_permission" "sns" {
  count = var.enable_lambda_processor ? 1 : 0

  statement_id  = "AllowExecutionFromSNS"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.order_processor[0].function_name
  principal     = "sns.amazonaws.com"
  source_arn    = module.messaging.sns_topic_arn
}

resource "aws_sns_topic_subscription" "lambda" {
  count = var.enable_lambda_processor ? 1 : 0

  topic_arn = module.messaging.sns_topic_arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.order_processor[0].arn
}
