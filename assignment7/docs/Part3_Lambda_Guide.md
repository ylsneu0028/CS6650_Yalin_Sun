# Part III: Lambda 替代 ECS Workers — 一步一步操作指南

## 目标

1. 部署一个订阅 Part II SNS topic 的 Lambda 函数（512MB、Go、3 秒“支付”延迟）。
2. 通过现有 Order API 发送 5～10 笔测试订单。
3. 在 CloudWatch 中观察 **Cold Start** 与 **Warm Start**（REPORT 行中是否有 Init Duration）。
4. 完成成本估算与“是否切换到 Lambda”的一小段结论。

**注意：** 不要用 Locust 对 Lambda 做高并发压测，以免触发 AWS 风控或误伤账号；本部分用少量 curl 请求即可。

---

## 第一步：构建 Lambda 部署包（Zip）

Lambda 使用 Go 编写，需要先在本地编译为 Linux 可执行文件并打成 zip，Terraform 会引用这个 zip。

1. 打开终端，进入项目根目录：
   ```bash
   cd /Users/pear/Desktop/CS6650_Yalin_Sun/assignment7
   ```

2. 进入 Lambda 代码目录并安装依赖、构建：
   ```bash
   cd lambda-processor
   go mod tidy
   make build
   ```

3. 确认生成 `deployment.zip`：
   ```bash
   ls -la deployment.zip
   ```

4. 回到项目根目录（后续 Terraform 从 `terraform` 目录执行）：
   ```bash
   cd /Users/pear/Desktop/CS6650_Yalin_Sun/assignment7
   ```

---

## 第二步：部署 Lambda（Terraform apply）

Part III 的 Terraform 已添加：Lambda 函数、执行角色、SNS 订阅（订阅你 Part II 的 SNS topic）。  
当前设计下，**SNS 会同时发给 SQS 和 Lambda**，即 ECS 和 Lambda 都会处理同一批订单。若你希望“仅用 Lambda 处理、不重复处理”，可将 ECS processor 的 `desired_count` 设为 0，或后续在 Terraform 里去掉 SQS 订阅。

1. 进入 terraform 目录：
   ```bash
   cd /Users/pear/Desktop/CS6650_Yalin_Sun/assignment7/terraform
   ```

2. 初始化（若尚未执行过）并应用：
   ```bash
   terraform init
   terraform plan
   ```
   确认会创建 Lambda 相关资源（如 `aws_lambda_function.order_processor`、`aws_sns_topic_subscription.lambda` 等）。

3. 执行部署：
   ```bash
   terraform apply
   ```
   提示时输入 `yes`。

4. 记下输出中的 **Receiver URL**（Part II 的 ALB 地址）和 **Lambda 函数名**：
   ```bash
   terraform output receiver_url
   terraform output lambda_order_processor_name
   ```
   例如：`receiver_url = "http://assignment7-ecommerce-alb-xxxx.us-west-2.elb.amazonaws.com"`，`lambda_order_processor_name = "assignment7-ecommerce-order-processor"`。

---

## 第三步：发送 5～10 笔测试订单

使用 Part II 的异步接口，把订单发到 SNS，由 Lambda（以及可选的 ECS）处理。

1. 将 `YOUR_ALB` 换成你的 `receiver_url` 的主机部分（不要带 `http://` 也可以，下面示例用完整 URL）：
   ```bash
   RECEIVER_URL=$(terraform output -raw receiver_url)
   ```

2. 发送多笔订单（示例 5 笔，可改 `order_id` 再发到 10 笔）：
   ```bash
   for i in 1 2 3 4 5; do
     curl -s -X POST "$RECEIVER_URL/orders/async" \
       -H "Content-Type: application/json" \
       -d "{\"order_id\":\"part3-test-$i\",\"customer_id\":$i,\"status\":\"pending\",\"items\":[{\"id\":1,\"name\":\"keyboard\",\"quantity\":1,\"price\":100}]}"
     echo ""
   done
   ```
   预期每次返回类似：`{"mode":"async","order_id":"part3-test-1","status":"accepted"}`。

---

## 第四步：在 CloudWatch 中观察 Cold Start / Warm Start

### 4.1 找到 Lambda 日志

1. 登录 **AWS Console**，Region 选 **us-west-2**（与你的 Terraform `aws_region` 一致）。
2. 顶部搜索 **CloudWatch**，进入 **CloudWatch**。
3. 左侧 **Logs** → **Log groups**。
4. 在搜索框输入：`/aws/lambda/`，找到你的 Lambda 的 log group，例如：  
   **`/aws/lambda/assignment7-ecommerce-order-processor`**
5. 点击该 log group，再点击 **Latest log stream**（或按 Last event time 排序后的第一个），进入一条 log stream。

### 4.2 识别 Cold Start（带 Init Duration 的 REPORT）

在 log 中搜索 **REPORT**。若某次调用是冷启动，会看到类似：

```text
REPORT RequestId: xyz... Duration: 3005ms Billed: 3079ms Memory: 512MB Init Duration: 73ms
```

- **Init Duration** 表示冷启动开销（例如 73ms）。
- 同一 REPORT 行里 **Duration** 是业务逻辑时间（约 3000ms 为 3 秒“支付”）。

### 4.3 识别 Warm Start（无 Init Duration 的 REPORT）

连续或短时间内再次调用时，通常不会再冷启动，REPORT 类似：

```text
REPORT RequestId: abc... Duration: 3001ms Billed: 3002ms Memory: 512MB
```

没有 **Init Duration** 即为 Warm Start。

### 4.4 回答作业中的问题

- **Cold start 发生频率？** 通常是：第一次请求、或空闲约 5～15 分钟后的第一次请求。
- **冷启动开销？** 例如 73ms / 3000ms ≈ 2.4%，对 3 秒的“支付”处理影响很小。
- **对 3 秒支付处理是否重要？** 一般可以接受；若业务对首请求延迟敏感，可考虑 Provisioned Concurrency 或保留少量 ECS。

---

## 第五步：成本估算（作业要求）

### Part II 当前成本（参考）

- 2 个 ECS 任务（receiver + processor）× 约 $8.50/月 ≈ **$17/月**（始终运行）。

### Lambda 定价（官方）

- **请求：** $0.20 / 百万次。
- **计算：** $0.0000166667 / GB-秒。
- **免费额度（每月）：** 100 万次请求 + 40 万 GB-秒。

### 示例：每月 1 万单，3 秒/单，512MB（0.5 GB）

- 请求：10,000 &lt; 100 万 → **$0**。
- GB-秒：10,000 × 3 × 0.5 = 15,000 &lt; 40 万 → **$0**。  
→ **月成本 = $0（在免费额度内）。**

### 何时 Lambda 成本约等于 $17/月？

- 免费额度内：40 万 GB-秒 ÷ 1.5（3s × 0.5GB）≈ **约 26.7 万单/月** 仍免费。
- 超过免费额度后，需约 **约 170 万次请求/月** 量级才接近 $17（仅作数量级参考，请按官方计算器复核）。

---

## 第六步：Trade-off 与“是否切换”的结论（一段话）

作业要求写 **一段话**：你的创业团队是否应该切换到 Lambda？理由是什么？

可参考要点（按你的观察改写）：

- **收益：** 零运维（无需队列深度告警、worker 扩缩容、队列超时调优、ECS 健康监控）；按需付费；自动扩展。
- **损失：** 无 SQS 队列（无法像 Part II 那样积压与重试）；SNS 仅有限重试（约 2 次）后丢弃；无批量处理能力；冷启动约 2～3% 开销。
- **建议句式：**  
  “Based on our observations, cold starts occurred [every first request / after ~5–15 min idle] with about [X]ms init overhead ([Y]% of 3s). For our expected volume ([Z] orders/month), Lambda stays within free tier, so cost is $0 vs ECS $17. We recommend [switching to Lambda / keeping ECS / hybrid] because [你的理由：例如更看重零运维与成本，或更看重队列与重试控制].”

---

## 检查清单（Demonstrate 要求）

在代码库和/或团队报告中请体现：

- [ ] **已部署的 Lambda**：订阅 Part II SNS topic，处理订单（3 秒延迟）。
- [ ] **CloudWatch 冷启动观察**：至少一张 REPORT 截图（带 Init Duration）与一张无 Init Duration 的 REPORT。
- [ ] **成本计算**：你的预期月单量下的 Lambda 成本（及与 ECS $17 的对比）。
- [ ] **是否切换的建议**：一段话 + 理由（冷启动、成本、SQS/重试取舍等）。

---

## 可选：仅用 Lambda 处理（不重复处理）

若你希望 SNS **只** 触发 Lambda、不再写入 SQS（即 ECS processor 收不到订单）：

1. 在 `terraform/modules/messaging/main.tf` 中注释或删除 `resource "aws_sns_topic_subscription" "this"`（SNS → SQS 的那段）。
2. 运行 `terraform apply`。
3. 之后所有 `/orders/async` 的订单只会由 Lambda 处理；ECS processor 可保留但将没有新消息。

若保留当前配置（SNS 同时订阅 SQS 和 Lambda），则两边都会处理同一批订单；可将 ECS 的 `desired_count` 设为 0 来“关掉” ECS 处理，仅用 Lambda。
