# Optional ECS task/execution role when your account has no LabRole (or other named role).

resource "aws_iam_role" "ecs_task" {
  count = var.ecs_iam_role_arn == "" && var.create_ecs_iam_role ? 1 : 0
  name  = "${replace(var.service_name, "_", "-")}-ecs-task"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = "sts:AssumeRole"
      Principal = {
        Service = "ecs-tasks.amazonaws.com"
      }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_execution" {
  count      = var.ecs_iam_role_arn == "" && var.create_ecs_iam_role ? 1 : 0
  role       = aws_iam_role.ecs_task[0].name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Step II: ECS task role needs DynamoDB API access when cart_backend = dynamodb
resource "aws_iam_role_policy" "ecs_dynamodb_cart" {
  count = var.ecs_iam_role_arn == "" && var.create_ecs_iam_role && var.cart_backend == "dynamodb" ? 1 : 0
  name  = "${replace(var.service_name, "_", "-")}-dynamodb-cart"
  role  = aws_iam_role.ecs_task[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "dynamodb:PutItem",
        "dynamodb:GetItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:Query"
      ]
      Resource = [
        module.dynamodb.table_arn,
        "${module.dynamodb.table_arn}/index/*"
      ]
    }]
  })
}
