#!/usr/bin/env python3
"""
Step III Part 0: verify mysql_test_results.json + dynamodb_test_results.json
(150 ops each: 50 create_cart, 50 add_items, 50 get_cart) and write
combined_results.json — single source for Step III analysis / charts.

Exit code 1 if verification fails.
"""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


REQUIRED_OPS = {"create_cart": 50, "add_items": 50, "get_cart": 50}
TOTAL_REQUIRED = sum(REQUIRED_OPS.values())


def load_rows(path: Path) -> list[dict[str, Any]]:
    with path.open(encoding="utf-8") as f:
        data = json.load(f)
    if not isinstance(data, list):
        raise ValueError(f"{path}: expected JSON array")
    return data


def verify_backend(label: str, rows: list[dict[str, Any]]) -> tuple[dict[str, Any], list[str]]:
    errors: list[str] = []
    if len(rows) != TOTAL_REQUIRED:
        errors.append(f"{label}: expected {TOTAL_REQUIRED} operations, got {len(rows)}")

    counts = Counter(r.get("operation") for r in rows)
    for op, need in REQUIRED_OPS.items():
        got = counts.get(op, 0)
        if got != need:
            errors.append(f"{label}: expected {need} `{op}`, got {got}")

    missing_keys = []
    for i, r in enumerate(rows):
        for k in ("operation", "response_time", "success", "status_code", "timestamp"):
            if k not in r:
                missing_keys.append(f"{label} row {i}: missing `{k}`")
    errors.extend(missing_keys[:5])
    if len(missing_keys) > 5:
        errors.append(f"{label}: ... and {len(missing_keys) - 5} more missing-key rows")

    successes = sum(1 for r in rows if r.get("success") is True)
    success_rate = (successes / len(rows) * 100.0) if rows else 0.0

    summary = {
        "label": label,
        "total_operations": len(rows),
        "by_operation": dict(counts),
        "success_count": successes,
        "success_rate_percent": round(success_rate, 4),
        "passed": len(errors) == 0,
    }
    return summary, errors


def main() -> None:
    p = argparse.ArgumentParser(description="Step III: merge + verify perf JSON files")
    p.add_argument(
        "--mysql",
        type=Path,
        default=Path("mysql_test_results.json"),
        help="Path to mysql_test_results.json",
    )
    p.add_argument(
        "--dynamodb",
        type=Path,
        default=Path("dynamodb_test_results.json"),
        help="Path to dynamodb_test_results.json",
    )
    p.add_argument(
        "--output",
        type=Path,
        default=Path("combined_results.json"),
        help="Output combined_results.json",
    )
    args = p.parse_args()

    try:
        mysql_rows = load_rows(args.mysql)
        dynamo_rows = load_rows(args.dynamodb)
    except (OSError, ValueError, json.JSONDecodeError) as e:
        print(f"ERROR: failed to load input: {e}", file=sys.stderr)
        sys.exit(1)

    m_sum, m_err = verify_backend("mysql", mysql_rows)
    d_sum, d_err = verify_backend("dynamodb", dynamo_rows)

    all_errors = m_err + d_err
    if all_errors:
        print("Verification FAILED:", file=sys.stderr)
        for e in all_errors:
            print(f"  - {e}", file=sys.stderr)
        sys.exit(1)

    records: list[dict[str, Any]] = []
    for r in mysql_rows:
        records.append({"backend": "mysql", **r})
    for r in dynamo_rows:
        records.append({"backend": "dynamodb", **r})

    out_obj = {
        "meta": {
            "description": "Homework 8 Step III — merged Step I (MySQL) and Step II (DynamoDB) perf test rows",
            "schema_version": 1,
            "generated_at_utc": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "sources": {
                "mysql": str(args.mysql.as_posix()),
                "dynamodb": str(args.dynamodb.as_posix()),
            },
            "verification": {
                "mysql": m_sum,
                "dynamodb": d_sum,
                "combined_record_count": len(records),
                "passed": True,
            },
        },
        "records": records,
    }

    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", encoding="utf-8") as f:
        json.dump(out_obj, f, indent=2)
        f.write("\n")

    print("OK:", args.output)
    print(f"  mysql rows: {len(mysql_rows)}, dynamodb rows: {len(dynamo_rows)}, combined: {len(records)}")


if __name__ == "__main__":
    main()
