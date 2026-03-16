# Phase 3 Results Summary — Flash Sale Load Test (Async)

## Test setup (same as Phase 1 Flash Sale)

- **Scenario:** Flash sale — 20 concurrent users, 60 seconds
- **Endpoint (Phase 3):** `POST /orders/async` (publish to SNS, return 202 Accepted)
- **Target:** Receiver service behind ALB (`assignment7-ecommerce-alb-...`)

---

## Phase 3 results (async)

- **Total requests:** 3,451  
- **Failures:** 0 (**100% acceptance rate**)  
- **Throughput:** 57.7 requests per second (RPS)  
- **Response times:** Median 40 ms, average 42.78 ms, 95th percentile 60 ms, max 183 ms  

The API responds in under 100 ms because it only enqueues the order and returns 202; payment is processed later by workers. So under the same flash sale load, every order is accepted and no request fails.

---

## Comparison with Phase 1 (sync)

| Metric            | Phase 1 (sync)     | Phase 3 (async)   |
|-------------------|--------------------|-------------------|
| Endpoint          | `POST /orders/sync`| `POST /orders/async` |
| Total requests    | 285                | 3,451             |
| RPS               | 5.2                | 57.7              |
| Failures          | 0%                 | 0%                |
| Median response   | 3,300 ms           | 40 ms             |
| 95th %ile         | 4,900 ms           | 60 ms             |

**Why the difference?**

- **Sync:** Each request waits for the 3-second payment check. With one payment “slot” every 3 seconds, throughput is limited to about 5–6 orders per second even with 20 users. Users see 3–5 second response times.
- **Async:** The API returns 202 in ~40 ms after sending the order to the queue. Throughput is no longer limited by the 3-second payment step, so we see ~57.7 RPS and 100% acceptance.

**How many times more orders did the async approach accept?**

- In the same 60-second flash sale window: **3,451 vs 285** → about **12× more orders** with the async design.
- In terms of throughput: **57.7 vs 5.2 RPS** → about **11× higher RPS**.

---

## Conclusion

Phase 3 shows that moving to an event-driven, async flow (API → SNS → SQS → worker) keeps **100% acceptance rate** under flash sale load while **greatly increasing** the number of orders the system can accept. Customers get a fast “order accepted” response instead of long waits or timeouts, and heavy work is done asynchronously by the Order Processor.
