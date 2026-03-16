#!/usr/bin/env bash
# Phase 3: Flash sale load test on POST /orders/async — 期望 100% 接受率
# 与 Phase 1 相同负载: 20 并发用户, 60 秒, 10 users/s 启动

set -e
cd "$(dirname "$0")"

HOST="${1:-}"
if [[ -z "$HOST" ]]; then
  if command -v terraform &>/dev/null; then
    HOST=$(cd terraform && terraform output -raw receiver_url 2>/dev/null || true)
  fi
  if [[ -z "$HOST" ]]; then
    echo "Usage: $0 <receiver_url>"
    echo "Example: $0 http://assignment7-ecommerce-alb-xxxx.us-west-2.elb.amazonaws.com"
    echo "Or run from repo root after 'terraform apply': $0 \$(cd terraform && terraform output -raw receiver_url)"
    exit 1
  fi
fi

echo "Target: $HOST"
echo "Load: 20 users, 60s, spawn rate 10/s (flash sale)"
echo "---"

locust -f locustfile_async.py \
  --host="$HOST" \
  --users 20 \
  --spawn-rate 10 \
  --run-time 60s \
  --headless \
  --html report_async_flash.html \
  --csv results_async_flash

echo ""
echo "Done. Check report_async_flash.html and results_async_flash_stats.csv for 100% success rate."
