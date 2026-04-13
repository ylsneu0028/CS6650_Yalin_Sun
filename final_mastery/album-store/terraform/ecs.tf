resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${var.project}"
  retention_in_days = 14
}

resource "aws_ecs_cluster" "main" {
  name = var.project
}

resource "aws_ecs_task_definition" "app" {
  family                   = var.project
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "4096"
  memory                   = "16384"
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([
    {
      name  = "album-store"
      image = "${aws_ecr_repository.app.repository_url}:latest"
      portMappings = [
        {
          containerPort = 8080
          protocol      = "tcp"
        }
      ]
      essential = true
      environment = [
        { name = "AWS_REGION", value = var.aws_region },
        { name = "ALBUMS_TABLE", value = aws_dynamodb_table.albums.name },
        { name = "PHOTOS_TABLE", value = aws_dynamodb_table.photos.name },
        { name = "S3_BUCKET", value = aws_s3_bucket.photos.bucket },
        { name = "PORT", value = "8080" },
        { name = "ACCESS_LOG", value = "0" },
        { name = "UPLOAD_CONCURRENCY", value = "64" },
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.app.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "ecs"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "app" {
  name            = var.project
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.app.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  # Allow time for image pull + container start before ALB marks targets unhealthy
  health_check_grace_period_seconds = 120

  network_configuration {
    subnets          = data.aws_subnets.default.ids
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.app.arn
    container_name   = "album-store"
    container_port   = 8080
  }

  depends_on = [aws_lb_listener.http]
}
