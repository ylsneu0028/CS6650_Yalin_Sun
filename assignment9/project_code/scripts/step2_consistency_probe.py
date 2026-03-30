#!/usr/bin/env python3
"""
Step II Part 3: simple read-after-write probes (create->get, add->get).
Prints per-step latency in ms. Uses same auth as perf tests.
"""

from __future__ import annotations

import argparse
import json
import ssl
import time
import urllib.error
import urllib.request


def http_json(method: str, url: str, headers: dict, body: dict | None, ctx: ssl.SSLContext, timeout: float):
    data = json.dumps(body).encode("utf-8") if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    for k, v in headers.items():
        req.add_header(k, v)
    if data:
        req.add_header("Content-Type", "application/json")
    t0 = time.perf_counter()
    try:
        with urllib.request.urlopen(req, context=ctx, timeout=timeout) as resp:
            raw = resp.read()
            return resp.getcode(), raw, (time.perf_counter() - t0) * 1000.0
    except urllib.error.HTTPError as e:
        raw = e.read() if e.fp else b""
        return e.code, raw, (time.perf_counter() - t0) * 1000.0


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--base-url", default="http://127.0.0.1:8080")
    p.add_argument("--api-key", default="test")
    p.add_argument("--timeout", type=float, default=30.0)
    p.add_argument("--repeat", type=int, default=20)
    args = p.parse_args()
    base = args.base_url.rstrip("/")
    headers = {"X-API-Key": args.api_key}
    ctx = ssl.create_default_context()

    for i in range(args.repeat):
        code, raw, t_create = http_json(
            "POST", f"{base}/shopping-carts", headers, {"customer_id": 9000 + i}, ctx, args.timeout
        )
        if code != 201:
            print(f"iter {i}: create failed code={code} body={raw!r}")
            continue
        cid = json.loads(raw.decode())["shopping_cart_id"]
        code2, _, t_get = http_json("GET", f"{base}/shopping-carts/{cid}", headers, None, ctx, args.timeout)
        code3, _, t_add = http_json(
            "POST",
            f"{base}/shopping-carts/{cid}/items",
            headers,
            {"product_id": 1, "quantity": 2},
            ctx,
            args.timeout,
        )
        code4, raw4, t_get2 = http_json("GET", f"{base}/shopping-carts/{cid}", headers, None, ctx, args.timeout)
        items = json.loads(raw4.decode()).get("items", []) if code4 == 200 else []
        print(
            f"iter {i}: create={t_create:.1f}ms get1={t_get:.1f}ms (code {code2}) "
            f"add={t_add:.1f}ms (code {code3}) get2={t_get2:.1f}ms (code {code4}) items={len(items)}"
        )


if __name__ == "__main__":
    main()
