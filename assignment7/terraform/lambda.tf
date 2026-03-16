# ------------------------------------------------------------------------------
# Part III: Lambda processor subscribed to SNS (replaces/supplements ECS workers)
# Run from assignment7: cd lambda-processor && make build  before terraform apply
# Uses LabRole as execution role (course account cannot create IAM roles).
# ------------------------------------------------------------------------------

resource "aws_lambda_function" "order_processor" {
  filename         = "${path.module}/../lambda-processor/deployment.zip"
  function_name    = "${var.project_name}-order-processor"
  role             = data.aws_iam_role.lab_role.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  memory_size      = 512
  timeout          = 10
  source_code_hash = filebase64sha256("${path.module}/../lambda-processor/deployment.zip")
  tags             = local.common_tags
}

# Allow SNS to invoke Lambda
resource "aws_lambda_permission" "sns" {
  statement_id  = "AllowExecutionFromSNS"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.order_processor.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = module.messaging.sns_topic_arn
}

# Subscribe Lambda to Part II SNS topic (no SQS needed for Lambda path)
resource "aws_sns_topic_subscription" "lambda" {
  topic_arn = module.messaging.sns_topic_arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.order_processor.arn
}
