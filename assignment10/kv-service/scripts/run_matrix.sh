#!/usr/bin/env bash
# Example: run 4 write ratios for one mode (adjust --requests / compose strategy first).
# Prereq: Leader–Follower stack up, followers on 18081–18084, leader on 8080.
set -euo pipefail
cd "$(dirname "$0")"

LEADER="${LEADER:-http://localhost:8080}"
FOLLOWERS="${FOLLOWERS:-http://localhost:18081,http://localhost:18082,http://localhost:18083,http://localhost:18084}"
ALL="${ALL:-http://localhost:8080,$FOLLOWERS}"
REQ="${REQ:-2500}"
CONC="${CONC:-40}"
KEYS="${KEYS:-40}"
mkdir -p ../results

for wr in 0.01 0.1 0.5 0.9; do
  tag=$(echo "$wr" | tr . _)
  python3 loadtest.py --mode lf_w5_r1 \
    --leader "$LEADER" --followers "$FOLLOWERS" \
    --write-ratio "$wr" --requests "$REQ" --concurrency "$CONC" --keys "$KEYS" \
    --out "../results/lf_w5_r1_wr${tag}.jsonl"
done

echo "Done. Plot with: python3 plot_results.py ../results/lf_w5_r1_wr*.jsonl -o ../results/figures_lf_w5_r1"
