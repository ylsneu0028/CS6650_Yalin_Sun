#!/usr/bin/env python3
"""
Plot latency distributions and read-after-write intervals from loadtest JSONL.

Usage:
  python plot_results.py results_lf_w5r1_wr50.jsonl -o figures/
  python plot_results.py runs/*.jsonl -o figures/
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np


def load_jsonl(path: Path) -> list[dict]:
    rows = []
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rows.append(json.loads(line))
    return rows


def percentile(xs: np.ndarray, p: float) -> float:
    if len(xs) == 0:
        return float("nan")
    return float(np.percentile(xs, p))


def plot_one(rows: list[dict], title: str, out_dir: Path, stem: str) -> None:
    reads = [r["latency_ms"] for r in rows if r.get("op") == "read" and r.get("latency_ms") is not None]
    writes = [r["latency_ms"] for r in rows if r.get("op") == "write" and r.get("latency_ms") is not None]
    deltas = [
        r["delta_ms"]
        for r in rows
        if r.get("op") == "read" and r.get("delta_ms") is not None and r["delta_ms"] >= 0
    ]

    read_arr = np.array(reads, dtype=float)
    write_arr = np.array(writes, dtype=float)
    delta_arr = np.array(deltas, dtype=float)

    out_dir.mkdir(parents=True, exist_ok=True)

    def fig_hist(data: np.ndarray, xlab: str, fname: str, log_x: bool = False) -> None:
        if len(data) == 0:
            return
        fig, ax = plt.subplots(figsize=(8, 4))
        ax.hist(data, bins=80, color="steelblue", edgecolor="white", alpha=0.85)
        ax.set_xlabel(xlab)
        ax.set_ylabel("count")
        ax.set_title(title)
        if log_x:
            ax.set_xscale("log")
        fig.tight_layout()
        fig.savefig(out_dir / fname, dpi=150)
        plt.close(fig)

    def fig_cdf(data: np.ndarray, xlab: str, fname: str) -> None:
        if len(data) == 0:
            return
        s = np.sort(data)
        y = np.arange(1, len(s) + 1) / len(s)
        fig, ax = plt.subplots(figsize=(8, 4))
        ax.plot(s, y, color="darkred", lw=1.5)
        ax.set_xlabel(xlab)
        ax.set_ylabel("CDF")
        ax.set_title(title + " (CDF)")
        ax.grid(True, alpha=0.3)
        fig.tight_layout()
        fig.savefig(out_dir / fname, dpi=150)
        plt.close(fig)

    fig_hist(read_arr, "read latency (ms)", f"{stem}_read_latency_hist.png")
    fig_cdf(read_arr, "read latency (ms)", f"{stem}_read_latency_cdf.png")
    fig_hist(write_arr, "write latency (ms)", f"{stem}_write_latency_hist.png")
    fig_cdf(write_arr, "write latency (ms)", f"{stem}_write_latency_cdf.png")
    if len(delta_arr) > 0:
        fig_hist(delta_arr, "time since last write to same key (ms)", f"{stem}_read_write_gap_hist.png")
        fig_hist(delta_arr, "time since last write (ms), log scale", f"{stem}_read_write_gap_hist_logx.png", log_x=True)
        fig_cdf(delta_arr, "time since last write (ms)", f"{stem}_read_write_gap_cdf.png")

    stats = {
        "read_p50": percentile(read_arr, 50),
        "read_p95": percentile(read_arr, 95),
        "read_p99": percentile(read_arr, 99),
        "write_p50": percentile(write_arr, 50),
        "write_p95": percentile(write_arr, 95),
        "write_p99": percentile(write_arr, 99),
        "stale_reads": sum(1 for r in rows if r.get("stale")),
        "read_samples": len(read_arr),
        "write_samples": len(write_arr),
    }
    with open(out_dir / f"{stem}_stats.json", "w", encoding="utf-8") as f:
        json.dump(stats, f, indent=2)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("jsonl", nargs="+", help="One or more JSONL files from loadtest.py")
    ap.add_argument("-o", "--out-dir", default="figures", help="Output directory for PNGs")
    args = ap.parse_args()

    out_dir = Path(args.out_dir)
    for jpath in args.jsonl:
        p = Path(jpath)
        stem = p.stem
        rows = load_jsonl(p)
        title = stem.replace("_", " ")
        plot_one(rows, title, out_dir, stem)
        print(f"plotted {p} -> {out_dir}/{stem}_*.png")


if __name__ == "__main__":
    main()
