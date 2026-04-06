#!/usr/bin/env python3
"""
KV load tester for CS6650 Assignment 10 Part V.

Leader–Follower:
  - Writes always go to --leader.
  - lf_w5_r1: reads use GET /local_read on a random follower (detects replica lag).
  - lf_w1_r5 / lf_w3_r3: reads use GET /get on a random replica from --all-replicas.

Leaderless:
  - Writes and reads use random nodes from --nodes; reads use GET /get (local R=1).

Stale read: after a successful write, client records X-KV-Version for that key.
If a later read returns a lower version (or missing value while we expect one), count stale.

Example (compose with follower ports 18081–18084, leader 8080):
  python loadtest.py --mode lf_w5_r1 \\
    --leader http://localhost:8080 \\
    --followers http://localhost:18081,http://localhost:18082,http://localhost:18083,http://localhost:18084 \\
    --write-ratio 0.5 --requests 3000 --concurrency 40 --keys 40 \\
    --out results_lf_w5r1_wr50.jsonl
"""

from __future__ import annotations

import argparse
import asyncio
import json
import random
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import httpx

VERSION_HEADER = "x-kv-version"


def parse_urls(s: str) -> list[str]:
    return [u.strip().rstrip("/") for u in s.split(",") if u.strip()]


@dataclass
class KeyState:
    last_write_version: int = 0
    last_write_time: float = 0.0
    ever_written: bool = False


@dataclass
class GlobalState:
    keys: dict[str, KeyState] = field(default_factory=dict)
    lock: asyncio.Lock = field(default_factory=asyncio.Lock)

    async def on_write_ok(self, key: str, version: int) -> None:
        async with self.lock:
            st = self.keys.setdefault(key, KeyState())
            st.last_write_version = max(st.last_write_version, version)
            st.last_write_time = time.perf_counter()
            st.ever_written = True

    async def get_write_version(self, key: str) -> int:
        async with self.lock:
            return self.keys.get(key, KeyState()).last_write_version

    async def get_last_write_time(self, key: str) -> float | None:
        async with self.lock:
            st = self.keys.get(key)
            if not st or not st.ever_written:
                return None
            return st.last_write_time


def key_name(i: int) -> str:
    return f"k{i}"


async def do_write(
    client: httpx.AsyncClient,
    leader_url: str,
    key: str,
    value: str,
) -> dict[str, Any]:
    t0 = time.perf_counter()
    try:
        r = await client.post(
            f"{leader_url}/set",
            json={"key": key, "value": value},
            timeout=120.0,
        )
    except Exception as e:
        t1 = time.perf_counter()
        return {
            "op": "write",
            "latency_ms": (t1 - t0) * 1000,
            "key": key,
            "status": 0,
            "error": str(e),
            "version": None,
            "stale": False,
            "delta_ms": None,
        }
    t1 = time.perf_counter()
    ver = None
    if r.headers.get(VERSION_HEADER):
        try:
            ver = int(r.headers[VERSION_HEADER])
        except ValueError:
            ver = None
    return {
        "op": "write",
        "latency_ms": (t1 - t0) * 1000,
        "key": key,
        "status": r.status_code,
        "version": ver,
        "stale": False,
        "delta_ms": None,
    }


async def do_get(
    client: httpx.AsyncClient,
    base_url: str,
    key: str,
    expected_version: int,
    last_write_time: float | None,
) -> dict[str, Any]:
    t0 = time.perf_counter()
    try:
        r = await client.get(f"{base_url}/get", params={"key": key}, timeout=120.0)
    except Exception as e:
        t1 = time.perf_counter()
        return {
            "op": "read",
            "latency_ms": (t1 - t0) * 1000,
            "key": key,
            "status": 0,
            "error": str(e),
            "version": None,
            "stale": False,
            "delta_ms": None,
        }
    t1 = time.perf_counter()
    ver = None
    if r.headers.get(VERSION_HEADER):
        try:
            ver = int(r.headers[VERSION_HEADER])
        except ValueError:
            ver = None
    stale = False
    if expected_version > 0 and ver is not None and ver < expected_version:
        stale = True
    if expected_version > 0 and r.status_code == 404:
        stale = True
    delta_ms = None
    if last_write_time is not None:
        delta_ms = (t0 - last_write_time) * 1000
    return {
        "op": "read",
        "latency_ms": (t1 - t0) * 1000,
        "key": key,
        "status": r.status_code,
        "version": ver,
        "stale": stale,
        "delta_ms": delta_ms,
    }


async def do_local_read(
    client: httpx.AsyncClient,
    base_url: str,
    key: str,
    expected_version: int,
    last_write_time: float | None,
) -> dict[str, Any]:
    t0 = time.perf_counter()
    try:
        r = await client.get(f"{base_url}/local_read", params={"key": key}, timeout=120.0)
    except Exception as e:
        t1 = time.perf_counter()
        return {
            "op": "read",
            "read_path": "local_read",
            "latency_ms": (t1 - t0) * 1000,
            "key": key,
            "status": 0,
            "error": str(e),
            "version": None,
            "stale": False,
            "delta_ms": None,
        }
    t1 = time.perf_counter()
    ver = None
    ok = False
    try:
        body = r.json()
        ok = bool(body.get("ok"))
        if ok:
            ver = int(body.get("version", 0))
    except Exception:
        body = {}
    stale = False
    if expected_version > 0:
        if not ok:
            stale = True
        elif ver is not None and ver < expected_version:
            stale = True
    delta_ms = None
    if last_write_time is not None:
        delta_ms = (t0 - last_write_time) * 1000
    return {
        "op": "read",
        "read_path": "local_read",
        "latency_ms": (t1 - t0) * 1000,
        "key": key,
        "status": r.status_code,
        "version": ver,
        "stale": stale,
        "delta_ms": delta_ms,
    }


async def one_request(
    client: httpx.AsyncClient,
    args: argparse.Namespace,
    state: GlobalState,
    key_ids: list[int],
) -> dict[str, Any]:
    kid = random.choice(key_ids)
    key = key_name(kid)
    write_roll = random.random() < args.write_ratio

    if args.mode == "leaderless":
        nodes = parse_urls(args.nodes)
        if write_roll:
            base = random.choice(nodes)
            val = f"v-{time.time_ns()}"
            rec = await do_write_to_node(client, base, key, val)
            if rec["status"] == 201 and rec.get("version") is not None:
                await state.on_write_ok(key, rec["version"])
            return rec
        base = random.choice(nodes)
        ev = await state.get_write_version(key)
        lwt = await state.get_last_write_time(key)
        rec = await do_get(client, base, key, ev, lwt)
        return rec

    # Leader–follower: writes only to leader
    leader = args.leader.rstrip("/")
    if write_roll:
        val = f"v-{time.time_ns()}"
        rec = await do_write(client, leader, key, val)
        if rec["status"] == 201 and rec.get("version") is not None:
            await state.on_write_ok(key, rec["version"])
        return rec

    ev = await state.get_write_version(key)
    lwt = await state.get_last_write_time(key)

    if args.mode == "lf_w5_r1":
        followers = parse_urls(args.followers)
        fb = random.choice(followers)
        return await do_local_read(client, fb, key, ev, lwt)

    replicas = parse_urls(args.all_replicas)
    rb = random.choice(replicas)
    return await do_get(client, rb, key, ev, lwt)


async def do_write_to_node(
    client: httpx.AsyncClient,
    base_url: str,
    key: str,
    value: str,
) -> dict[str, Any]:
    t0 = time.perf_counter()
    try:
        r = await client.post(
            f"{base_url.rstrip('/')}/set",
            json={"key": key, "value": value},
            timeout=120.0,
        )
    except Exception as e:
        t1 = time.perf_counter()
        return {
            "op": "write",
            "latency_ms": (t1 - t0) * 1000,
            "key": key,
            "status": 0,
            "error": str(e),
            "version": None,
            "stale": False,
            "delta_ms": None,
        }
    t1 = time.perf_counter()
    ver = None
    if r.headers.get(VERSION_HEADER):
        try:
            ver = int(r.headers[VERSION_HEADER])
        except ValueError:
            ver = None
    return {
        "op": "write",
        "latency_ms": (t1 - t0) * 1000,
        "key": key,
        "status": r.status_code,
        "version": ver,
        "stale": False,
        "delta_ms": None,
    }


async def run_load(args: argparse.Namespace) -> list[dict[str, Any]]:
    key_ids = list(range(args.keys))
    state = GlobalState()
    sem = asyncio.Semaphore(args.concurrency)
    results: list[dict[str, Any]] = []

    async with httpx.AsyncClient() as client:

        async def wrapped(i: int) -> None:
            async with sem:
                rec = await one_request(client, args, state, key_ids)
                rec["seq"] = i
                rec["ts"] = time.time()
                results.append(rec)

        await asyncio.gather(*(wrapped(i) for i in range(args.requests)))

    return results


def main() -> None:
    ap = argparse.ArgumentParser(description="KV load test client")
    ap.add_argument("--mode", required=True, choices=["lf_w5_r1", "lf_w1_r5", "lf_w3_r3", "leaderless"])
    ap.add_argument("--leader", default="", help="Leader base URL (leader–follower)")
    ap.add_argument(
        "--followers",
        default="",
        help="Comma-separated follower URLs for lf_w5_r1 local_read",
    )
    ap.add_argument(
        "--all-replicas",
        default="",
        help="Comma URLs (leader+followers) for lf_w1_r5 / lf_w3_r3 GET reads",
    )
    ap.add_argument("--nodes", default="", help="Comma URLs for leaderless mode")
    ap.add_argument("--write-ratio", type=float, required=True, help="Probability of write, e.g. 0.01")
    ap.add_argument("--requests", type=int, default=2000)
    ap.add_argument("--concurrency", type=int, default=30)
    ap.add_argument("--keys", type=int, default=40, help="Key space size k0..k{N-1} (local-in-time)")
    ap.add_argument("--out", required=True, help="Output JSONL path")
    ap.add_argument("--seed", type=int, default=None)
    args = ap.parse_args()

    if args.seed is not None:
        random.seed(args.seed)

    if args.mode != "leaderless":
        if not args.leader:
            ap.error("--leader required for leader–follower modes")
        if args.mode == "lf_w5_r1" and not args.followers:
            ap.error("--followers required for lf_w5_r1")
        if args.mode in ("lf_w1_r5", "lf_w3_r3") and not args.all_replicas:
            ap.error("--all-replicas required for lf_w1_r5 and lf_w3_r3")
    else:
        if not args.nodes:
            ap.error("--nodes required for leaderless")

    results = asyncio.run(run_load(args))

    reads = [r for r in results if r["op"] == "read"]
    writes = [r for r in results if r["op"] == "write"]
    stale = sum(1 for r in reads if r.get("stale"))
    ok_write = sum(1 for r in writes if r.get("status") == 201)

    summary = {
        "mode": args.mode,
        "write_ratio": args.write_ratio,
        "requests": args.requests,
        "concurrency": args.concurrency,
        "keys": args.keys,
        "read_count": len(reads),
        "write_count": len(writes),
        "writes_201": ok_write,
        "stale_reads": stale,
        "stale_rate_reads": stale / len(reads) if reads else 0.0,
    }
    summary_path = args.out.replace(".jsonl", "_summary.json")
    if summary_path == args.out:
        summary_path = args.out + ".summary.json"
    out_p = Path(args.out)
    out_p.parent.mkdir(parents=True, exist_ok=True)
    Path(summary_path).parent.mkdir(parents=True, exist_ok=True)
    with open(args.out, "w", encoding="utf-8") as f:
        for r in results:
            f.write(json.dumps(r) + "\n")
    with open(summary_path, "w", encoding="utf-8") as f:
        json.dump(summary, f, indent=2)

    print(json.dumps(summary, indent=2))
    print(f"wrote {args.out} and {summary_path}", flush=True)


if __name__ == "__main__":
    main()
