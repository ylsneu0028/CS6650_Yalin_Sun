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

module "ecs" {
  source                  = "./modules/ecs"
  service_name            = var.service_name
  image                   = "${module.ecr.repository_url}:latest"
  container_port          = var.container_port
  subnet_ids              = module.network.public_subnet_ids
  security_group_ids      = [module.network.security_group_id]
  execution_role_arn      = local.ecs_role_arn
  task_role_arn           = local.ecs_role_arn
  log_group_name          = module.logging.log_group_name
  ecs_count               = var.ecs_count
  region                  = var.aws_region
  container_environment   = local.ecs_container_environment
}

resource "docker_image" "app" {
  name = "${module.ecr.repository_url}:latest"
  build {
    context    = "../src"
    dockerfile = "Dockerfile"
    platform   = "linux/arm64"
  }
}

resource "docker_registry_image" "app" {
  name = docker_image.app.name
}
