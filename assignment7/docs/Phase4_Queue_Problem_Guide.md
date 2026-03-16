# Phase 4: The Queue Problem — 操作指南

## 目标

1. 在 CloudWatch 里看到 SQS 队列深度（ApproximateNumberOfMessagesVisible）在 flash sale 测试期间快速上升。
2. 填写作业里的分析：队列增长率、清空积压所需时间。

---

## 第一步：确认你的 SQS 队列名称和 Region

1. 打开终端，进入 terraform 目录：
   ```bash
   cd /Users/pear/Desktop/CS6650_Yalin_Sun/assignment7/terraform
   ```
2. 查看队列 URL（里面有队列名和 region）：
   ```bash
   terraform output sqs_queue_url
   ```
   例如输出：`https://sqs.us-west-2.amazonaws.com/257810805238/order-processing-queue`  
   则 **队列名** = `order-processing-queue`，**Region** = `us-west-2`。

记下这两个，后面在 AWS 控制台会用到。

---

## 第二步：再跑一次 Flash Sale 测试（让队列里有流量）

为了在 CloudWatch 里看到明显的队列积压，需要先产生负载：

1. 启动 Locust（或用之前的脚本）：
   ```bash
   cd /Users/pear/Desktop/CS6650_Yalin_Sun/assignment7
   RECEIVER_URL=$(cd terraform && terraform output -raw receiver_url)
   locust -f locustfile_async.py --host="$RECEIVER_URL"
   ```
2. 浏览器打开：http://localhost:8089  
3. 设置：
   - **Number of users:** 20  
   - **Spawn rate:** 10  
4. 点击 **Start**，跑满 **60 秒**，然后点击 **Stop**。  
   （这样大约会以 ~60/s 的速率往 SNS/SQS 送订单。）

---

## 第三步：在 CloudWatch 看 SQS 队列深度

### 3.1 打开 CloudWatch 并找到 SQS 指标

1. 登录 **AWS Console**，右上角 **Region** 选 **us-west-2**（和你 `sqs_queue_url` 的 region 一致）。
2. 顶部搜索框输入 **CloudWatch**，进入 **CloudWatch**。
3. 左侧菜单点击 **Metrics** → **All metrics**。
4. 在 **Metrics** 页面，找到 **SQS** 分区，点击 **SQS** → **Queue Metrics**。

### 3.2 选择你的队列和指标

1. 在 **Queue Metrics** 里，在 **Select metric** 区域：
   - **Metric name** 选：**ApproximateNumberOfMessagesVisible**  
     （表示当前“可见”、即可被消费者取走的消息条数。）
2. 在 **Dimensions** 里选你的队列：
   - **Queue Name** 选：`order-processing-queue`（或你 terraform 里配置的队列名）。
3. 点击 **Graphed metrics** 标签，确认画的是 **ApproximateNumberOfMessagesVisible**。
4. 右上角 **Time range** 选 **Last 1 hour**（或包含你刚才跑 Locust 的那段时间）。

此时你会看到一条曲线：在 flash sale 的 60 秒内，这条线会**快速上升**；测试结束后，因为只有一个 worker（每 3 秒处理 1 条），曲线会**非常缓慢**下降。这就是 “Queue Problem”。

可选：再选一个指标 **ApproximateNumberOfMessagesNotVisible**（正在被处理、未可见的消息数），可以一起看。

### 3.3 截图留证

- 截一张图：**ApproximateNumberOfMessagesVisible** 在测试期间明显上升、测试结束后缓慢下降。  
作业/报告里用这张图说明 “queue buildup during flash sale”。

---

## 第四步：填写作业里的分析

### 4.1 已知条件（作业已给）

- **Order acceptance rate:** ~60/second（API 每秒接受约 60 个订单）
- **Single worker processing rate:** 1 order per 3 seconds = **0.33/second**

### 4.2 队列增长率（Queue growth rate）

公式：  
**Queue growth rate = Order acceptance rate − Processing rate**

- Order acceptance rate ≈ 60 messages/second  
- Processing rate = 1/3 ≈ 0.33 messages/second  

所以：  
**Queue growth rate ≈ 60 − 0.33 ≈ 59.67 messages/second**  
（可写 **~60 messages/second** 或 **≈ 59.7 msg/s**。）

含义：在 flash sale 期间，每秒大约多出 60 条消息积压在队列里。

### 4.3 清空积压所需时间（Time to clear backlog）

两种方式任选一种或都写：

**方式 A：用你从 CloudWatch 看到的“峰值可见消息数”**

1. 在 CloudWatch 图上，看 **ApproximateNumberOfMessagesVisible** 在 flash sale 结束后的**峰值**（约等于 60 秒内积压的消息数）。
2. 例如峰值约为 **N** 条（例如 3500–3600）。
3. 清空时间（秒）= **N ÷ 0.33**  
   清空时间（分钟）= **N ÷ 0.33 ÷ 60**  
   例如 N = 3600：3600 ÷ 0.33 ≈ 10909 秒 ≈ **182 分钟**（约 3 小时）。

**方式 B：用理论值（不查图也可写）**

- 60 秒内接受约 60×60 = 3600 条消息。  
- 单 worker 处理速率 0.33/s。  
- 清空时间 ≈ 3600 ÷ 0.33 秒 ≈ 10909 秒 ≈ **182 分钟**（约 **3 小时**）。

作业填空示例：

- **Queue growth rate:** ~60 messages/second（或 59.7 msg/s）。  
- **Time to clear backlog:** 约 180 分钟（约 3 小时）；若你从 CloudWatch 读了实际峰值 N，可写：N 条 ÷ 0.33/s ≈ ___ 分钟。

### 4.4 一句话总结（Customer service 那句话）

可以这样写：  
**Customer service is getting calls: "Where's my order confirmation?"**  
因为订单虽然被“接受”了（202），但实际处理要等队列慢慢消化，单 worker 下要数小时才能处理完，所以用户迟迟收不到确认。

---

## 第五步：可选 — 从 CLI 看当前队列属性

在终端可以快速看队列当前可见消息数（与 CloudWatch 对应）：

```bash
aws sqs get-queue-attributes \
  --queue-url "https://sqs.us-west-2.amazonaws.com/257810805238/order-processing-queue" \
  --attribute-names ApproximateNumberOfMessages ApproximateNumberOfMessagesNotVisible
```

把 `queue-url` 换成你的 `terraform output sqs_queue_url` 输出。  
**ApproximateNumberOfMessages** 就是当前“可见”消息数，和 CloudWatch 的 ApproximateNumberOfMessagesVisible 一致。

---

## 检查清单（交作业前确认）

- [ ] 在 CloudWatch → Metrics → SQS → Queue Metrics 中查看了 **ApproximateNumberOfMessagesVisible**。
- [ ] 时间范围包含一次 flash sale 测试（20 users, 60s），并看到曲线明显上升。
- [ ] 已截图：队列深度在测试中上升、测试后缓慢下降。
- [ ] 分析中已填写：  
  - Queue growth rate: **~60 messages/second**  
  - Time to clear backlog: **约 180 分钟（或你根据实际峰值算出的分钟数）**
- [ ] 已用一句话说明为何会出现 “Where’s my order confirmation?” 的客诉（订单已接受但处理严重滞后）。

完成以上步骤即完成 Phase 4 的操作与分析。
