#!/usr/bin/env bash
# Build, push :latest to ECR, and force a new ECS deployment.
# Run from the album-store directory after `terraform apply`.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

REGION="${AWS_REGION:-us-west-2}"
TF_DIR="$ROOT/terraform"

REPO_URL="$(terraform -chdir="$TF_DIR" output -raw ecr_repository_url)"
CLUSTER="$(terraform -chdir="$TF_DIR" output -raw ecs_cluster_name)"
SERVICE="$(terraform -chdir="$TF_DIR" output -raw ecs_service_name)"

if ! terraform -chdir="$TF_DIR" output ecr_repository_url >/dev/null 2>&1; then
  echo "Run terraform apply in terraform/ first." >&2
  exit 1
fi

aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "${REPO_URL%%/*}"

# Fargate uses linux/amd64. On Apple Silicon, plain "docker build" produces arm64 → CannotPullContainerError.
docker buildx build \
  --platform linux/amd64 \
  -t "$REPO_URL:latest" \
  --push \
  .

aws ecs update-service \
  --region "$REGION" \
  --cluster "$CLUSTER" \
  --service "$SERVICE" \
  --force-new-deployment \
  >/dev/null

echo "Pushed $REPO_URL:latest and triggered rollout on $CLUSTER / $SERVICE"
