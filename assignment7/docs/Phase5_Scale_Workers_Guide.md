# Phase 5: Scale Your Workers — 操作指南

---

## 开始前：要不要 terraform destroy？

**不需要。** 不需要 destroy 再 apply，原因如下：

1. **队列积压**：之前测试留下的 SQS 消息会干扰新测试的指标（队列深度、清空时间）。
2. **正确做法**：在**每次** Phase 5 的测试前 **清空 SQS 队列（Purge）**，让每次 flash sale 都从 0 条消息开始，这样：
   - 峰值队列深度只反映本次 60 秒的负载；
   - “队列何时回到 0” 只反映当前 worker 数的处理速度；
   - 截图和数字不会被旧消息影响。

**操作：每次跑新的 worker 配置（1 / 5 / 20 / 100）前执行一次 Purge（见下文步骤 2）。**

---

## 准备工作

### 1. 记下队列 URL 和 Region

```bash
cd /Users/pear/Desktop/CS6650_Yalin_Sun/assignment7/terraform
terraform output -raw sqs_queue_url
# 例如: https://sqs.us-west-2.amazonaws.com/257810805238/order-processing-queue
# Region = us-west-2（从 URL 或 AWS Console 确认）
```

### 2. 清空 SQS 队列（每次新测试前执行）

```bash
# 把 <YOUR_QUEUE_URL> 换成上面输出的 URL
aws sqs purge-queue --queue-url "https://sqs.us-west-2.amazonaws.com/257810805238/order-processing-queue" --region us-west-2
```

- Purge 后队列会立刻变为 0 条（可能有几秒延迟）。
- **等 1～2 分钟** 再开始 Locust，这样 CloudWatch 和 ECS 状态更稳定。

### 3. 修改 Worker 数量并部署

Worker 数由 Terraform 变量 `processor_worker_count` 控制，改完后需要 apply 让 ECS 用新任务定义重启 processor。

**方式 A：命令行传参（推荐）**

```bash
cd /Users/pear/Desktop/CS6650_Yalin_Sun/assignment7/terraform

# 例如先测 5 个 worker
terraform apply -var="processor_worker_count=5" -auto-approve
```

**方式 B：改 tfvars 或 variables.tf 的 default**

在 `terraform.tfvars` 里加一行（没有这个文件就新建）：

```hcl
processor_worker_count = 5
```

然后执行：

```bash
terraform apply -auto-approve
```

每次要换 worker 数（1 / 5 / 20 / 100）时，改这个值并重新 apply 即可。

---

## 单次测试流程（对每个 worker 数重复）

下面以 **5 workers** 为例；把 “5” 换成 1、20、100 即可重复。

### Step 1：设为当前要测的 worker 数并部署

```bash
cd /Users/pear/Desktop/CS6650_Yalin_Sun/assignment7/terraform
terraform apply -var="processor_worker_count=5" -auto-approve
```

等到 apply 完成，ECS 会滚动更新 processor 服务（约 1～2 分钟）。

### Step 2：清空队列并短暂等待

```bash
aws sqs purge-queue --queue-url "https://sqs.us-west-2.amazonaws.com/257810805238/order-processing-queue" --region us-west-2
# 等待 1～2 分钟
sleep 120
```

### Step 3：打开 CloudWatch 图表（便于边跑边看）

1. AWS Console → **CloudWatch** → **Metrics** → **SQS** → **Queue Metrics**
2. 选 **ApproximateNumberOfMessagesVisible**，Queue 选 `order-processing-queue`
3. 时间范围选 **1h** 或 **3h**，保持这个页面开着，方便后面截图

### Step 4：跑 Flash Sale（与 Phase 3/4 相同）

```bash
cd /Users/pear/Desktop/CS6650_Yalin_Sun/assignment7
RECEIVER_URL=$(cd terraform && terraform output -raw receiver_url)
locust -f locustfile_async.py --host="$RECEIVER_URL"
```

- 浏览器打开 http://localhost:8089  
- **Number of users:** 20  
- **Spawn rate:** 10  
- 点 **Start**，跑满 **60 秒**，点 **Stop**

### Step 5：记录并截图

- **Peak queue depth（峰值队列深度）**  
  - 在 CloudWatch 图上看 **ApproximateNumberOfMessagesVisible** 在 60 秒内的最高点（单位：条）。  
  - 截一张图（包含这条曲线和峰值），在报告里标出 “Peak queue depth = ___”.

- **Time until queue returns to zero（队列回到 0 的时间）**  
  - 测试结束后继续看同一张图，看曲线从峰值降到 0 用了多少分钟。  
  - 可选：在 CloudWatch 把时间范围拉长到 3h，观察曲线何时归零。

- **Resource utilization（可选）**  
  - CloudWatch → **Metrics** → **ECS** → **ClusterName, ServiceName**  
  - 选 **CPUUtilization**、**MemoryUtilization**，对应 processor 服务，截一张图或记下大致范围。

### Step 6：填表

| Worker 数 | Processing rate (orders/s) | Peak queue depth | Time until queue = 0 | 备注 |
|-----------|----------------------------|------------------|----------------------|------|
| 1         | 0.33                       | （你的截图值）   | （你的观察）         | Phase 3 已有 |
| 5         | 见下方公式                 |                  |                      |      |
| 20        | 见下方公式                 |                  |                      |      |
| 100       | 见下方公式                 |                  |                      |      |

**Processing rate（理论值）：**  
每个 worker 每 3 秒处理 1 单 ⇒ 1 worker = 1/3 ≈ **0.33 orders/s**

- 5 workers: 5 × 0.33 ≈ **1.67 orders/s**
- 20 workers: 20 × 0.33 ≈ **6.67 orders/s**
- 100 workers: 100 × 0.33 ≈ **33.33 orders/s**

（若你按 “总处理条数 / 清空时间” 算实际速率，也可以一并写在报告里。）

---

## 四种配置的完整顺序建议

1. **1 worker**（当前默认）  
   - 若 Phase 4 已测过且做过 Purge，可跳过；否则：Purge → 跑 60s flash sale → 记峰值、归零时间。
2. **5 workers**  
   - `terraform apply -var="processor_worker_count=5"` → Purge → 等 1～2 min → 跑 Locust 60s → 记录 + 截图。
3. **20 workers**  
   - `terraform apply -var="processor_worker_count=20"` → Purge → 等 1～2 min → 跑 Locust 60s → 记录 + 截图。
4. **100 workers**  
   - `terraform apply -var="processor_worker_count=100"` → Purge → 等 1～2 min → 跑 Locust 60s → 记录 + 截图。

每次换 worker 数都要 **apply → Purge → 再跑 Locust**，这样数据互不干扰。

---

## 最小 worker 数：防止 60 orders/s 时队列积压

- 要**不积压**，需要：**处理速率 ≥ 入队速率**  
- 入队速率 ≈ **60 orders/s**（flash sale）  
- 每 worker 处理速率 = **1/3 ≈ 0.33 orders/s**  
- 所以需要：**worker 数 ≥ 60 ÷ 0.33 ≈ 182**

**结论：最小 worker 数约为 180～200**（可写 “about 180” 或 “≈182”）。  
若要验证，可设 `processor_worker_count=200`，Purge 后再跑一次 60s flash sale，看 CloudWatch 里队列是否还会持续上升；若大致稳定或缓慢下降，即说明足够。

---

## 检查清单（交作业前）

- [ ] 每次新 worker 配置测试前都执行了 **Purge SQS**，没有用 destroy。
- [ ] 对 1 / 5 / 20 / 100 四种 worker 数都做了：apply → Purge → 60s flash sale。
- [ ] 记录了每种配置的：**Peak queue depth**、**Time until queue returns to zero**，并附 CloudWatch 截图。
- [ ] 填好了 **Processing rate** 表（理论值或实测值）。
- [ ] 可选：记录了 **Resource utilization**（CPU/Memory）截图。
- [ ] 写了 **最小 worker 数**（约 180～200）及简要推导或验证说明。

完成以上步骤即完成 Phase 5 的操作与文档要求。
