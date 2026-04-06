#!/usr/bin/env python3
"""
Build comparison CDF figures for the assignment PDF: for each read/write ratio,
overlay all four configurations (LF W5R1, LF W1R5, LF W3R3, leaderless) on
read latency, write latency, and read-after-write interval.

Run from kv-service root:
  python scripts/plot_report_comparisons.py

Reads JSONL from results/<config>/<stem>.jsonl, writes PNGs to results/figures/report/.
"""

from __future__ import annotations

import json
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np

CONFIGS: list[tuple[str, str]] = [
    ("lf_w5_r1", "LF W=5, R=1"),
    ("lf_w1_r5", "LF W=1, R=5"),
    ("lf_w3_r3", "LF W=3, R=3"),
    ("leaderless", "Leaderless (W=N, R=1)"),
]

STEMS: list[tuple[str, str]] = [
    ("wr001", "1% writes / 99% reads"),
    ("wr010", "10% writes / 90% reads"),
    ("wr050", "50% writes / 50% reads"),
    ("wr090", "90% writes / 10% reads"),
]

COLORS = ["#1f77b4", "#ff7f0e", "#2ca02c", "#d62728"]


def load_jsonl(path: Path) -> list[dict]:
    rows = []
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rows.append(json.loads(line))
    return rows


def cdf_xy(data: np.ndarray) -> tuple[np.ndarray, np.ndarray]:
    if len(data) == 0:
        return np.array([]), np.array([])
    s = np.sort(data)
    y = np.arange(1, len(s) + 1) / len(s)
    return s, y


def plot_overlay(
    series: list[tuple[str, np.ndarray]],
    xlabel: str,
    title: str,
    out_path: Path,
) -> None:
    fig, ax = plt.subplots(figsize=(9, 5))
    for (label, arr), color in zip(series, COLORS):
        if len(arr) == 0:
            continue
        x, y = cdf_xy(arr)
        ax.plot(x, y, lw=2, label=label, color=color)
    ax.set_xlabel(xlabel)
    ax.set_ylabel("CDF")
    ax.set_title(title)
    ax.grid(True, alpha=0.3)
    ax.legend(loc="lower right", fontsize=9)
    fig.tight_layout()
    out_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(out_path, dpi=160)
    plt.close(fig)


def extract_arrays(rows: list[dict]) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    reads = [r["latency_ms"] for r in rows if r.get("op") == "read" and r.get("latency_ms") is not None]
    writes = [r["latency_ms"] for r in rows if r.get("op") == "write" and r.get("latency_ms") is not None]
    deltas = [
        r["delta_ms"]
        for r in rows
        if r.get("op") == "read" and r.get("delta_ms") is not None and r["delta_ms"] >= 0
    ]
    return (
        np.array(reads, dtype=float),
        np.array(writes, dtype=float),
        np.array(deltas, dtype=float),
    )


def main() -> None:
    root = Path(__file__).resolve().parent.parent
    results_dir = root / "results"
    out_dir = results_dir / "figures" / "report"

    for stem, ratio_title in STEMS:
        read_series: list[tuple[str, np.ndarray]] = []
        write_series: list[tuple[str, np.ndarray]] = []
        gap_series: list[tuple[str, np.ndarray]] = []

        for cfg_dir, label in CONFIGS:
            jpath = results_dir / cfg_dir / f"{stem}.jsonl"
            if not jpath.is_file():
                print(f"skip missing {jpath}")
                continue
            rows = load_jsonl(jpath)
            ra, wa, da = extract_arrays(rows)
            read_series.append((label, ra))
            write_series.append((label, wa))
            gap_series.append((label, da))

        base = f"{stem}_comparison"
        plot_overlay(
            read_series,
            "Read latency (ms)",
            f"Read latency CDF — {ratio_title}",
            out_dir / f"{base}_reads_cdf.png",
        )
        plot_overlay(
            write_series,
            "Write latency (ms)",
            f"Write latency CDF — {ratio_title}",
            out_dir / f"{base}_writes_cdf.png",
        )
        plot_overlay(
            gap_series,
            "Time since last write to same key (ms)",
            f"Read–write interval CDF — {ratio_title}",
            out_dir / f"{base}_read_write_gap_cdf.png",
        )
        print(f"wrote {base}_*.png -> {out_dir}")

    print("done.")


if __name__ == "__main__":
    main()
