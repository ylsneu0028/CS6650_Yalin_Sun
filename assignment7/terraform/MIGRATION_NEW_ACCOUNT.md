# 从旧账号 (2578...) 迁移到新账号 (8195...)

## 原因

- `terraform.tfstate` 里记录的是**旧账号**里的资源 ARN。
- 用**新账号**凭证跑 Terraform 时会出现 `AccountIDs mismatch`、跨账号访问 403、ECR 登录账号不一致等错误。

**正确做法**：在新账号里**重新创建**一整套资源，并使用**新账号的 state**。

---

## 1. 登录新账号并确认身份

```bash
# 若使用 SSO（示例）
aws sso login --profile <你的profile名>
export AWS_PROFILE=<你的profile名>

aws sts get-caller-identity
```

**Account** 必须是 **`819551470797`**（或你当前要用的账号）。

---

## 2. 备份并弃用旧 state（重要）

在 `terraform/` 目录：

```bash
cd terraform
mv terraform.tfstate terraform.tfstate.backup.old_account
mv terraform.tfstate.backup terraform.tfstate.backup.old_account2 2>/dev/null || true
```

之后 Terraform 会认为「从零开始」，只会在**当前登录账号**里创建资源。

> 旧账号 2578 若已无法登录，**无法**自动 `terraform destroy` 旧资源，旧资源可能仍计费直到平台回收；以课程/云厂商说明为准。

---

## 3. 配置 IAM 角色名 `ecs_iam_role_name`

旧环境里的 **`LabRole`** 在新账号里**往往不存在**。

在 **IAM → Roles** 里找一个能用于 ECS Fargate 的角色，或**新建**一个角色：

- **Trust**：`ecs-tasks.amazonaws.com`
- **权限**（可合并到同一角色，与作业里「一个角色兼 execution + task」一致）需包含：
  - 拉 ECR 镜像、写 CloudWatch Logs（execution）
  - `sns:Publish`（receiver 发 SNS）
  - `sqs:ReceiveMessage`, `sqs:DeleteMessage`, `sqs:GetQueueAttributes`（processor 消费 SQS）

在 `terraform.tfvars` 里增加一行（**改成你的角色名**）：

```hcl
ecs_iam_role_name = "你的ECS任务角色名称"
```

若变量未设置，默认仍为 `LabRole`（仅当该名在新账号存在时才能用）。

---

## 4. Lambda（可选）

默认 **`enable_lambda_processor = false`**，避免缺少 `lambda-processor/deployment.zip` 或 Lambda 执行角色策略导致 apply 失败。

需要 Part III Lambda 时：

1. `cd ../lambda-processor && make build` 生成 `deployment.zip`
2. 确认 `ecs_iam_role_name` 对应角色**允许 Lambda 使用**（或单独拆 Lambda 角色并改 `lambda.tf`）
3. 在 `terraform.tfvars` 中设置：`enable_lambda_processor = true`

---

## 5. 初始化并部署

```bash
cd terraform
terraform init -upgrade
terraform plan
terraform apply
```

Docker 会往**当前账号**的 ECR 推镜像；确保本机 Docker 已运行。

---

## 6. 更新本地 `terraform.tfvars` 示例

可参考仓库中的 `terraform.tfvars.example`（若已添加），不要把密钥写进 Git。

---

## 常见问题

| 现象 | 处理 |
|------|------|
| `couldn't find resource`（IAM role） | 检查 `ecs_iam_role_name` 是否与新账号 IAM 中名称完全一致 |
| ECR / Docker 认证失败 | 确认 `aws sts get-caller-identity` 已是新账号，且 `terraform apply` 在同一终端会话 |
| SNS/SQS 403 | 任务角色缺少对应 IAM 策略 |
