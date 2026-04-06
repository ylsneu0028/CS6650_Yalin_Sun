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

data "aws_iam_role" "ecs" {
  count = var.ecs_iam_role_arn == "" && !var.create_ecs_iam_role ? 1 : 0
  name  = var.ecs_iam_role_name
}

locals {
  ecs_role_arn = var.ecs_iam_role_arn != "" ? var.ecs_iam_role_arn : (
    var.create_ecs_iam_role ? aws_iam_role.ecs_task[0].arn : data.aws_iam_role.ecs[0].arn
  )

  ecs_container_environment = concat(
    [
      { name = "PORT", value = tostring(var.container_port) },
      { name = "KV_ROLE", value = var.kv_role },
      { name = "KV_PEER_URLS", value = var.kv_peer_urls },
      { name = "KV_N", value = tostring(var.kv_n) },
      { name = "KV_R", value = tostring(var.kv_r) },
      { name = "KV_W", value = tostring(var.kv_w) },
    ],
    var.kv_all_urls != "" ? [{ name = "KV_ALL_URLS", value = var.kv_all_urls }] : [],
    var.kv_leader_url != "" ? [{ name = "KV_LEADER_URL", value = var.kv_leader_url }] : [],
    var.kv_self_url != "" ? [{ name = "KV_SELF_URL", value = var.kv_self_url }] : [],
  )
}
