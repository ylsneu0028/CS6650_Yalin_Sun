#!/usr/bin/env python3
"""
Runs exactly 150 API operations (50 create, 50 add items, 50 get cart) and writes
mysql_test_results.json for Homework 8 Step I.

Uses only the Python standard library (no pip install required).
"""

from __future__ import annotations

import argparse
import json
import ssl
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from typing import Any


def utc_timestamp() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def http_json(
    method: str,
    url: str,
    headers: dict[str, str],
    body: dict[str, Any] | None,
    ctx: ssl.SSLContext,
    timeout: float,
) -> tuple[int, bytes, float]:
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, method=method)
    for k, v in headers.items():
        req.add_header(k, v)
    if data is not None:
        req.add_header("Content-Type", "application/json")

    start = time.perf_counter()
    try:
        with urllib.request.urlopen(req, context=ctx, timeout=timeout) as resp:
            raw = resp.read()
            ms = (time.perf_counter() - start) * 1000.0
            return resp.getcode(), raw, ms
    except urllib.error.HTTPError as e:
        raw = e.read() if e.fp else b""
        ms = (time.perf_counter() - start) * 1000.0
        return e.code, raw, ms
    except urllib.error.URLError:
        ms = (time.perf_counter() - start) * 1000.0
        return 0, b"", ms


def main() -> None:
    p = argparse.ArgumentParser(description="MySQL cart API 150-operation perf test")
    p.add_argument(
        "--base-url",
        default="http://127.0.0.1:8080",
        help="API base URL without trailing slash (e.g. http://alb-dns:8080)",
    )
    p.add_argument(
        "--api-key",
        default="test",
        help="Value for X-API-Key header (must match what the API expects)",
    )
    p.add_argument(
        "--output",
        default="mysql_test_results.json",
        help="Output JSON file path",
    )
    p.add_argument("--timeout", type=float, default=60.0)
    args = p.parse_args()

    base = args.base_url.rstrip("/")
    headers = {"X-API-Key": args.api_key}
    ctx = ssl.create_default_context()
    results: list[dict[str, Any]] = []

    cart_ids: list[int] = []

    for i in range(50):
        url = f"{base}/shopping-carts"
        code, raw, ms = http_json(
            "POST", url, headers, {"customer_id": i + 1}, ctx, args.timeout
        )
        ok = code == 201
        cart_id = None
        if ok:
            try:
                cart_id = int(json.loads(raw.decode("utf-8"))["shopping_cart_id"])
            except (ValueError, KeyError, json.JSONDecodeError):
                ok = False
        if cart_id is not None:
            cart_ids.append(cart_id)
        results.append(
            {
                "operation": "create_cart",
                "response_time": round(ms, 3),
                "success": ok,
                "status_code": code,
                "timestamp": utc_timestamp(),
            }
        )

    for idx, cart_id in enumerate(cart_ids):
        url = f"{base}/shopping-carts/{cart_id}/items"
        product_id = (idx % 5) + 1
        code, _, ms = http_json(
            "POST",
            url,
            headers,
            {"product_id": product_id, "quantity": 1},
            ctx,
            args.timeout,
        )
        ok = code == 204
        results.append(
            {
                "operation": "add_items",
                "response_time": round(ms, 3),
                "success": ok,
                "status_code": code,
                "timestamp": utc_timestamp(),
            }
        )

    for cart_id in cart_ids:
        url = f"{base}/shopping-carts/{cart_id}"
        code, _, ms = http_json("GET", url, headers, None, ctx, args.timeout)
        ok = code == 200
        results.append(
            {
                "operation": "get_cart",
                "response_time": round(ms, 3),
                "success": ok,
                "status_code": code,
                "timestamp": utc_timestamp(),
            }
        )

    with open(args.output, "w", encoding="utf-8") as f:
        json.dump(results, f, indent=2)
        f.write("\n")

    failed = sum(1 for r in results if not r["success"])
    print(f"Wrote {args.output} ({len(results)} ops, {failed} failures)")


if __name__ == "__main__":
    main()
