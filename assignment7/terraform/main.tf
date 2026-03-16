locals {
  common_tags = {
    Project = var.project_name
    Course  = "CS6650"
    Phase   = "phase3"
  }
}

module "network" {
  source = "./modules/network"

  project_name = var.project_name
  aws_region   = var.aws_region
  tags         = local.common_tags
}

module "receiver_ecr" {
  source = "./modules/ecr"

  repository_name = "${var.project_name}-receiver"
  tags            = local.common_tags
}

module "processor_ecr" {
  source = "./modules/ecr"

  repository_name = "${var.project_name}-processor"
  tags            = local.common_tags
}

module "receiver_logging" {
  source = "./modules/logging"

  log_group_name    = "/ecs/${var.project_name}-receiver"
  retention_in_days = var.log_retention_days
  tags              = local.common_tags
}

module "processor_logging" {
  source = "./modules/logging"

  log_group_name    = "/ecs/${var.project_name}-processor"
  retention_in_days = var.log_retention_days
  tags              = local.common_tags
}

module "messaging" {
  source = "./modules/messaging"

  topic_name = "order-processing-events"
  queue_name = "order-processing-queue"
  tags       = local.common_tags
}

module "alb" {
  source = "./modules/alb"

  project_name          = var.project_name
  vpc_id                = module.network.vpc_id
  public_subnet_ids     = module.network.public_subnet_ids
  alb_security_group_id = module.network.alb_security_group_id
  target_port           = var.receiver_container_port
  health_check_path     = "/health"
  tags                  = local.common_tags
}

data "aws_iam_role" "ecs_task" {
  name = var.ecs_iam_role_name
}

resource "aws_ecs_cluster" "main" {
  name = "${var.project_name}-cluster"
  tags = local.common_tags
}

resource "docker_image" "receiver" {
  name = "${module.receiver_ecr.repository_url}:${var.receiver_image_tag}"

  build {
    context    = coalesce(var.receiver_source_dir, "${path.module}/../receiver")
    dockerfile = "Dockerfile"
  }
}

resource "docker_registry_image" "receiver" {
  name = docker_image.receiver.name
}

resource "docker_image" "processor" {
  name = "${module.processor_ecr.repository_url}:${var.processor_image_tag}"

  build {
    context    = coalesce(var.processor_source_dir, "${path.module}/../processor")
    dockerfile = "Dockerfile"
  }
}

resource "docker_registry_image" "processor" {
  name = docker_image.processor.name
}

module "receiver_service" {
  source = "./modules/ecs-service"

  project_name            = var.project_name
  service_name            = "receiver"
  cluster_name            = aws_ecs_cluster.main.name
  cpu                     = var.receiver_cpu
  memory                  = var.receiver_memory
  desired_count           = var.receiver_desired_count
  task_execution_role_arn = data.aws_iam_role.ecs_task.arn
  task_role_arn           = data.aws_iam_role.ecs_task.arn
  image                   = docker_registry_image.receiver.name
  container_name          = "receiver"
  container_port          = var.receiver_container_port
  subnet_ids              = module.network.private_subnet_ids
  security_group_ids      = [module.network.receiver_security_group_id]
  assign_public_ip        = false
  log_group_name          = module.receiver_logging.log_group_name
  aws_region              = var.aws_region
  environment = [
    {
      name  = "APP_MODE"
      value = "receiver"
    },
    {
      name  = "AWS_REGION"
      value = var.aws_region
    },
    {
      name  = "SNS_TOPIC_ARN"
      value = module.messaging.sns_topic_arn
    }
  ]
  target_group_arn = module.alb.target_group_arn
  tags             = local.common_tags

  depends_on = [docker_registry_image.receiver]
}

module "processor_service" {
  source = "./modules/ecs-service"

  project_name            = var.project_name
  service_name            = "processor"
  cluster_name            = aws_ecs_cluster.main.name
  cpu                     = var.processor_cpu
  memory                  = var.processor_memory
  desired_count           = var.processor_desired_count
  task_execution_role_arn = data.aws_iam_role.ecs_task.arn
  task_role_arn           = data.aws_iam_role.ecs_task.arn
  image                   = docker_registry_image.processor.name
  container_name          = "processor"
  container_port          = 8080
  subnet_ids              = module.network.private_subnet_ids
  security_group_ids      = [module.network.processor_security_group_id]
  assign_public_ip        = false
  log_group_name          = module.processor_logging.log_group_name
  aws_region              = var.aws_region
  environment = [
    {
      name  = "APP_MODE"
      value = "processor"
    },
    {
      name  = "AWS_REGION"
      value = var.aws_region
    },
    {
      name  = "SQS_QUEUE_URL"
      value = module.messaging.sqs_queue_url
    },
    {
      name  = "WORKER_COUNT"
      value = tostring(var.processor_worker_count)
    }
  ]
  target_group_arn = null
  tags             = local.common_tags

  depends_on = [docker_registry_image.processor]
}