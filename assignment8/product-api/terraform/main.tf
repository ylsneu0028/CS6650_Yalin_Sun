# Wire together four focused modules: network, ecr, logging, ecs.

module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
  vpc_id         = var.vpc_id
}

module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

locals {
  effective_private_subnet_ids = length(var.private_subnet_ids) > 0 ? var.private_subnet_ids : module.network.private_subnet_ids
}

module "dynamodb" {
  source      = "./modules/dynamodb"
  name_prefix = replace(var.service_name, "_", "-")
}

module "rds" {
  source = "./modules/rds"

  name_prefix                = replace(var.service_name, "_", "-")
  vpc_id                     = module.network.vpc_id
  subnet_ids                 = local.effective_private_subnet_ids
  allowed_security_group_ids = [module.network.security_group_id]
  database_name              = var.db_name
  master_username            = var.db_username
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
      { name = "DB_HOST", value = module.rds.address },
      { name = "DB_PORT", value = tostring(module.rds.port) },
      { name = "DB_USER", value = module.rds.master_username },
      { name = "DB_PASSWORD", value = module.rds.master_password },
      { name = "DB_NAME", value = module.rds.database_name },
    ],
    var.cart_backend == "dynamodb" ? [
      { name = "CART_BACKEND", value = "dynamodb" },
      { name = "DYNAMODB_TABLE_NAME", value = module.dynamodb.table_name },
      { name = "AWS_REGION", value = var.aws_region },
    ] : [{ name = "CART_BACKEND", value = "mysql" }]
  )
}

module "ecs" {
  source             = "./modules/ecs"
  service_name       = var.service_name
  image              = "${module.ecr.repository_url}:latest"
  container_port     = var.container_port
  # ECS must use public subnets only; including RDS private subnets breaks inbound access.
  subnet_ids         = module.network.public_subnet_ids
  security_group_ids = [module.network.security_group_id]
  execution_role_arn = local.ecs_role_arn
  task_role_arn      = local.ecs_role_arn
  log_group_name     = module.logging.log_group_name
  ecs_count          = var.ecs_count
  region             = var.aws_region
  container_environment = local.ecs_container_environment
}


// Build & push the Go app image into ECR
resource "docker_image" "app" {
  # Use the URL from the ecr module, and tag it "latest"
  name = "${module.ecr.repository_url}:latest"

  build {
    # relative path from terraform/ → src/
    context = "../src"
    # Dockerfile defaults to "Dockerfile" in that context
  }
}

resource "docker_registry_image" "app" {
  # this will push :latest → ECR
  name = docker_image.app.name
}
