resource "aws_sns_topic" "this" {
  name = var.topic_name
  tags = var.tags
}

resource "aws_sqs_queue" "this" {
  name                       = var.queue_name
  receive_wait_time_seconds  = 20
  visibility_timeout_seconds = 30
  message_retention_seconds  = 345600
  tags                       = var.tags
}

data "aws_iam_policy_document" "queue_policy" {
  statement {
    sid    = "AllowSNSToSendMessage"
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["sns.amazonaws.com"]
    }

    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.this.arn]

    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = [aws_sns_topic.this.arn]
    }
  }
}

resource "aws_sqs_queue_policy" "this" {
  queue_url = aws_sqs_queue.this.id
  policy    = data.aws_iam_policy_document.queue_policy.json
}

resource "aws_sns_topic_subscription" "this" {
  topic_arn = aws_sns_topic.this.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.this.arn
}