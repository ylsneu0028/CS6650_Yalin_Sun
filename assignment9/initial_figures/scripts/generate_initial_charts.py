#!/usr/bin/env python3
"""
Generate preliminary charts for: Scalable LLM-backed RAG Q&A system.

Data below are ILLUSTRATIVE placeholders for the assignment item
"Initial charts and graphs for results". Replace arrays with measured values
after you run load tests and index-size experiments.

Usage:
  pip install -r requirements-charts.txt
  python scripts/generate_initial_charts.py

Outputs (PNG + PDF) under ../initial_figures/
"""

from __future__ import annotations

import argparse
from pathlib import Path

import matplotlib.pyplot as plt
import matplotlib.ticker as mticker

# -----------------------------------------------------------------------------
# Illustrative data — edit these after experiments
# -----------------------------------------------------------------------------

# Experiment 1: concurrent users -> latency & errors
E1_USERS = [10, 50, 200]
E1_MEAN_MS = [420, 890, 2400]
E1_P95_MS = [780, 1800, 5100]
E1_ERROR_PCT = [0.0, 0.2, 2.1]

# Experiment 2: index size (chunk count) -> latency & memory
E2_CHUNKS = [1_000, 10_000, 100_000]
E2_LABELS = ["1k", "10k", "100k"]
E2_RETRIEVE_MS = [8, 25, 95]
E2_E2E_MS = [450, 520, 720]
E2_MEMORY_GB = [0.05, 0.4, 3.2]

# Experiment 3: replica count -> throughput & latency
E3_INSTANCES = [1, 2, 4]
E3_RPS = [12, 22, 38]
E3_MEAN_MS = [980, 620, 480]
E3_P95_MS = [2100, 1400, 1100]


def _setup_style() -> None:
    plt.rcParams.update(
        {
            "figure.figsize": (8, 5),
            "figure.dpi": 120,
            "font.size": 11,
            "axes.titlesize": 12,
            "axes.labelsize": 11,
            "legend.fontsize": 10,
            "axes.grid": True,
            "grid.alpha": 0.35,
        }
    )


def _save_fig(fig: plt.Figure, out_dir: Path, stem: str, formats: list[str]) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    for fmt in formats:
        path = out_dir / f"{stem}.{fmt}"
        fig.savefig(path, bbox_inches="tight", dpi=150 if fmt == "png" else None)
        print(f"Wrote {path}")


def _watermark_preliminary(fig: plt.Figure) -> None:
    fig.text(
        0.99,
        0.01,
        "Preliminary / illustrative data",
        ha="right",
        va="bottom",
        fontsize=8,
        color="0.35",
        style="italic",
    )


def fig1_concurrency(out_dir: Path, formats: list[str]) -> None:
    fig, ax1 = plt.subplots()
    ax1.plot(E1_USERS, E1_MEAN_MS, "o-", label="Mean latency", linewidth=2, markersize=8)
    ax1.plot(E1_USERS, E1_P95_MS, "s--", label="P95 latency", linewidth=2, markersize=8)
    ax1.set_xlabel("Concurrent users")
    ax1.set_ylabel("Latency (ms)")
    ax1.set_title("Experiment 1: Query latency vs. concurrent load")
    ax1.set_xticks(E1_USERS)
    ax1.legend(loc="upper left")

    ax2 = ax1.twinx()
    ax2.plot(
        E1_USERS,
        E1_ERROR_PCT,
        "D-",
        color="C3",
        linewidth=2,
        markersize=7,
        label="Error / timeout rate (%)",
    )
    ax2.set_ylabel("Error / timeout rate (%)")
    ax2.set_ylim(0, max(5, max(E1_ERROR_PCT) * 1.25))
    ax2.legend(loc="upper right")

    _watermark_preliminary(fig)
    fig.tight_layout()
    _save_fig(fig, out_dir, "fig1_concurrency_latency", formats)
    plt.close(fig)


def fig2_index_size(out_dir: Path, formats: list[str]) -> None:
    fig, ax1 = plt.subplots()
    x = range(len(E2_LABELS))
    ax1.plot(x, E2_RETRIEVE_MS, "o-", label="Retrieve latency", linewidth=2, markersize=8)
    ax1.plot(x, E2_E2E_MS, "s--", label="End-to-end latency", linewidth=2, markersize=8)
    ax1.set_xticks(list(x))
    ax1.set_xticklabels(E2_LABELS)
    ax1.set_xlabel("Number of document chunks in vector index")
    ax1.set_ylabel("Latency (ms)")
    ax1.set_title("Experiment 2: Latency vs. vector index size")
    ax1.legend(loc="upper left")

    ax2 = ax1.twinx()
    ax2.plot(x, E2_MEMORY_GB, "^:", color="C4", label="Approx. index RAM (GB)", linewidth=2, markersize=8)
    ax2.set_ylabel("Approx. index memory (GB)")
    ax2.legend(loc="center right")

    _watermark_preliminary(fig)
    fig.tight_layout()
    _save_fig(fig, out_dir, "fig2_index_size_latency", formats)
    plt.close(fig)


def fig3_horizontal_scaling(out_dir: Path, formats: list[str]) -> None:
    fig, ax1 = plt.subplots()
    x = list(range(len(E3_INSTANCES)))
    labels = [str(n) for n in E3_INSTANCES]
    ax1.bar(x, E3_RPS, color="C0", alpha=0.75, label="Throughput (RPS)")
    ax1.set_xticks(x)
    ax1.set_xticklabels(labels)
    ax1.set_xlabel("Number of service instances (scaled tier)")
    ax1.set_ylabel("Throughput (requests / second)")
    ax1.set_title("Experiment 3: Throughput and latency vs. horizontal scaling")
    ax1.legend(loc="upper left")
    ax1.yaxis.set_major_locator(mticker.MaxNLocator(integer=True))

    ax2 = ax1.twinx()
    ax2.plot(
        x,
        E3_MEAN_MS,
        "o-",
        color="C1",
        linewidth=2,
        markersize=8,
        label="Mean latency (ms)",
    )
    ax2.plot(
        x,
        E3_P95_MS,
        "s--",
        color="C2",
        linewidth=2,
        markersize=8,
        label="P95 latency (ms)",
    )
    ax2.set_ylabel("Latency (ms)")
    ax2.legend(loc="upper right")

    _watermark_preliminary(fig)
    fig.tight_layout()
    _save_fig(fig, out_dir, "fig3_horizontal_scaling", formats)
    plt.close(fig)


def fig4_latency_breakdown(out_dir: Path, formats: list[str]) -> None:
    """Optional: stacked bar for one nominal request (hypothesis / pilot)."""
    stages = ["Embed", "Retrieve", "LLM"]
    ms = [35, 12, 890]
    colors = ["#4C72B0", "#55A868", "#C44E52"]
    fig, ax = plt.subplots()
    left = 0
    for stage, m, c in zip(stages, ms, colors):
        ax.barh(0, m, left=left, height=0.45, label=stage, color=c)
        ax.text(left + m / 2, 0, f"{stage}\n{m} ms", ha="center", va="center", fontsize=9, color="white", fontweight="bold")
        left += m
    ax.set_yticks([])
    ax.set_xlabel("Time (ms)")
    ax.set_title("Illustrative single-request latency breakdown (one nominal query)")
    ax.set_xlim(0, sum(ms) * 1.02)
    ax.legend(loc="upper center", bbox_to_anchor=(0.5, -0.12), ncol=3)
    _watermark_preliminary(fig)
    fig.tight_layout()
    _save_fig(fig, out_dir, "fig4_latency_breakdown_illustrative", formats)
    plt.close(fig)


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate initial RAG scalability charts.")
    parser.add_argument(
        "--out",
        type=Path,
        default=Path(__file__).resolve().parent.parent / "initial_figures",
        help="Output directory for figures",
    )
    parser.add_argument(
        "--formats",
        nargs="+",
        default=["png", "pdf"],
        choices=["png", "pdf"],
        help="File formats to write",
    )
    parser.add_argument("--no-breakdown", action="store_true", help="Skip optional fig4")
    args = parser.parse_args()

    _setup_style()
    out_dir: Path = args.out

    fig1_concurrency(out_dir, args.formats)
    fig2_index_size(out_dir, args.formats)
    fig3_horizontal_scaling(out_dir, args.formats)
    if not args.no_breakdown:
        fig4_latency_breakdown(out_dir, args.formats)

    print(f"\nDone. Figures in: {out_dir.resolve()}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
