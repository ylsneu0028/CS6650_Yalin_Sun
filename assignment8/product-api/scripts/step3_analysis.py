#!/usr/bin/env python3
"""
Step III: load combined_results.json and print Part 1 comparison tables
(avg, p50, p95, p99, success rate, per-operation avgs). All numbers trace to records[].

Usage (from product-api/):
  python3 scripts/step3_analysis.py
  python3 scripts/step3_analysis.py --input combined_results.json --format markdown
"""

from __future__ import annotations

import argparse
import json
import sys
from collections import defaultdict
from pathlib import Path
from statistics import mean
from typing import Any


def pct(xs: list[float], p: float) -> float:
    xs = sorted(xs)
    if not xs:
        return 0.0
    k = (len(xs) - 1) * p / 100.0
    f = int(k)
    c = min(f + 1, len(xs) - 1)
    return xs[f] + (k - f) * (xs[c] - xs[f])


def main() -> None:
    ap = argparse.ArgumentParser(description="Step III analysis from combined_results.json")
    ap.add_argument("--input", type=Path, default=Path("combined_results.json"))
    ap.add_argument("--format", choices=("text", "markdown"), default="text")
    args = ap.parse_args()

    try:
        with args.input.open(encoding="utf-8") as f:
            doc = json.load(f)
    except (OSError, json.JSONDecodeError) as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)

    records = doc.get("records")
    if not isinstance(records, list):
        print("ERROR: combined file missing `records` array", file=sys.stderr)
        sys.exit(1)

    by_backend: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for r in records:
        b = r.get("backend")
        if b not in ("mysql", "dynamodb"):
            print(f"ERROR: bad backend in row: {b!r}", file=sys.stderr)
            sys.exit(1)
        by_backend[str(b)].append(r)

    def stats_for(rows: list[dict[str, Any]]) -> dict[str, Any]:
        times = [float(r["response_time"]) for r in rows]
        succ = sum(1 for r in rows if r.get("success") is True)
        n = len(rows)
        return {
            "n": n,
            "avg": mean(times) if times else 0.0,
            "p50": pct(times, 50),
            "p95": pct(times, 95),
            "p99": pct(times, 99),
            "success_rate": (succ / n * 100.0) if n else 0.0,
        }

    mysql = by_backend["mysql"]
    dynamo = by_backend["dynamodb"]
    sm, sd = stats_for(mysql), stats_for(dynamo)

    def winner(metric: str, lower_is_better: bool) -> str:
        vm, vd = sm[metric], sd[metric]
        if vm == vd:
            return "tie"
        if lower_is_better:
            return "MySQL" if vm < vd else "DynamoDB"
        return "MySQL" if vm > vd else "DynamoDB"

    def margin(metric: str, lower_is_better: bool) -> str:
        vm, vd = sm[metric], sd[metric]
        d = abs(vm - vd)
        if lower_is_better:
            if vm < vd:
                return f"MySQL faster by {d:.2f} ms"
            if vd < vm:
                return f"DynamoDB faster by {d:.2f} ms"
        else:
            if vm > vd:
                return f"MySQL +{d:.2f} pp"
            if vd > vm:
                return f"DynamoDB +{d:.2f} pp"
        return "0"

    op_rows: dict[str, dict[str, float]] = defaultdict(dict)
    for op in ("create_cart", "add_items", "get_cart"):
        for label, rows in ("mysql", mysql), ("dynamodb", dynamo):
            sub = [r for r in rows if r.get("operation") == op]
            op_rows[op][label] = mean(float(r["response_time"]) for r in sub) if sub else 0.0

    data_source = "combined_results.json"

    if args.format == "markdown":
        print(f"**Data source:** `{data_source}` (`meta.verification` + `records`)")
        print()
        print("### Part 1 — Performance comparison")
        print()
        print("| Metric | MySQL | DynamoDB | Winner | Margin |")
        print("|--------|-------|----------|--------|--------|")
        print(
            f"| Avg Response Time (ms) | {sm['avg']:.2f} | {sd['avg']:.2f} | "
            f"{winner('avg', True)} | {margin('avg', True)} |"
        )
        print(
            f"| P50 Response Time (ms) | {sm['p50']:.2f} | {sd['p50']:.2f} | "
            f"{winner('p50', True)} | {margin('p50', True)} |"
        )
        print(
            f"| P95 Response Time (ms) | {sm['p95']:.2f} | {sd['p95']:.2f} | "
            f"{winner('p95', True)} | {margin('p95', True)} |"
        )
        print(
            f"| P99 Response Time (ms) | {sm['p99']:.2f} | {sd['p99']:.2f} | "
            f"{winner('p99', True)} | {margin('p99', True)} |"
        )
        print(
            f"| Success Rate (%) | {sm['success_rate']:.4f} | {sd['success_rate']:.4f} | "
            f"{winner('success_rate', False)} | {margin('success_rate', False)} |"
        )
        print("| Total Operations | 150 | 150 | — | — |")
        print()
        print("### Operation-specific breakdown")
        print()
        print("| Operation | MySQL Avg (ms) | DynamoDB Avg (ms) | Faster by |")
        print("|-----------|----------------|-------------------|-----------|")
        for op in ("create_cart", "add_items", "get_cart"):
            m, d = op_rows[op]["mysql"], op_rows[op]["dynamodb"]
            if m < d:
                by = f"MySQL ~{d - m:.2f} ms"
            elif d < m:
                by = f"DynamoDB ~{m - d:.2f} ms"
            else:
                by = "tie"
            print(f"| {op.upper()} | {m:.2f} | {d:.2f} | {by} |")
        return

    print("Data source:", data_source)
    print("=" * 76)
    print(f"{'Metric':<28} {'MySQL':>12} {'DynamoDB':>12} {'Winner':>10} {'Margin':>20}")
    print("-" * 76)
    rows_meta = [
        ("Avg Response Time (ms)", "avg", True),
        ("P50 Response Time (ms)", "p50", True),
        ("P95 Response Time (ms)", "p95", True),
        ("P99 Response Time (ms)", "p99", True),
        ("Success Rate (%)", "success_rate", False),
    ]
    for name, key, low in rows_meta:
        print(
            f"{name:<28} {sm[key]:>12.2f} {sd[key]:>12.2f} "
            f"{winner(key, low):>10} {margin(key, low):>20}"
        )
    print(f"{'Total Operations':<28} {'150':>12} {'150':>12} {'—':>10} {'—':>20}")
    print("=" * 76)
    print()
    print("Operation-specific (avg ms)")
    print("-" * 60)
    print(f"{'Operation':<14} {'MySQL':>12} {'DynamoDB':>12} {'Faster by':>18}")
    for op in ("create_cart", "add_items", "get_cart"):
        m, d = op_rows[op]["mysql"], op_rows[op]["dynamodb"]
        if m < d:
            by = f"MySQL ~{d-m:.2f} ms"
        elif d < m:
            by = f"DynamoDB ~{m-d:.2f} ms"
        else:
            by = "tie"
        print(f"{op:<14} {m:>12.2f} {d:>12.2f} {by:>18}")
    print()
    print("Cite in report: all values computed from `records` in", data_source)


if __name__ == "__main__":
    main()
