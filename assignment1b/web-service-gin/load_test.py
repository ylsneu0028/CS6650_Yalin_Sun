import argparse
import time
import requests
import numpy as np
import matplotlib.pyplot as plt
import csv
from datetime import datetime


def load_test(url: str, duration_seconds: int = 30, timeout_seconds: int = 10, sleep_ms: int = 0):
    """
    Sends repeated GET requests for duration_seconds.
    Records response time (ms), status code, and timestamp offset (s).
    """
    results = []
    start_time = time.time()
    end_time = start_time + duration_seconds

    print(f"Target URL: {url}")
    print(f"Starting load test for {duration_seconds} seconds...")

    req_count = 0
    while time.time() < end_time:
        req_count += 1
        t0 = time.time()
        try:
            r = requests.get(url, timeout=timeout_seconds)
            ok = True
            status = r.status_code
        except requests.exceptions.RequestException:
            ok = False
            status = None
        t1 = time.time()

        rt_ms = (t1 - t0) * 1000.0
        elapsed_s = t1 - start_time
        results.append((req_count, elapsed_s, rt_ms, ok, status))

        if ok and status == 200:
            print(f"Request {req_count}: {rt_ms:.2f} ms (200)")
        elif ok:
            print(f"Request {req_count}: {rt_ms:.2f} ms (status={status})")
        else:
            print(f"Request {req_count}: {rt_ms:.2f} ms (FAILED)")

        if sleep_ms > 0:
            time.sleep(sleep_ms / 1000.0)

    return results


def save_csv(results, filename: str):
    with open(filename, "w", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(["request_num", "elapsed_s", "response_time_ms", "ok", "status_code"])
        writer.writerows(results)


def plot_results(results, prefix: str):
    # Only plot successful requests’ response times
    success_times = [rt for (_, _, rt, ok, status) in results if ok and status is not None]

    if not success_times:
        print("No successful responses to plot.")
        return

    # Histogram
    plt.figure(figsize=(12, 6))
    plt.hist(success_times, bins=50, alpha=0.7)
    plt.xlabel("Response Time (ms)")
    plt.ylabel("Frequency")
    plt.title("Distribution of Response Times (Successful Requests)")
    hist_path = f"{prefix}_hist.png"
    plt.tight_layout()
    plt.savefig(hist_path, dpi=150)
    plt.close()

    # Scatter plot (request_num vs response_time)
    req_nums = []
    rts = []
    for (n, _, rt, ok, status) in results:
        if ok and status is not None:
            req_nums.append(n)
            rts.append(rt)

    plt.figure(figsize=(12, 6))
    plt.scatter(req_nums, rts, alpha=0.6)
    plt.xlabel("Request Number")
    plt.ylabel("Response Time (ms)")
    plt.title("Response Times Over Requests (Successful Requests)")
    scatter_path = f"{prefix}_scatter.png"
    plt.tight_layout()
    plt.savefig(scatter_path, dpi=150)
    plt.close()

    print(f"Saved plots: {hist_path}, {scatter_path}")


def print_stats(results):
    success_times = [rt for (_, _, rt, ok, status) in results if ok and status is not None]
    total = len(results)
    success = len(success_times)
    failed = total - success

    print("\nStatistics:")
    print(f"Total requests attempted: {total}")
    print(f"Successful responses:     {success}")
    print(f"Failed requests:          {failed}")

    if success_times:
        arr = np.array(success_times)
        print(f"Average response time:    {arr.mean():.2f} ms")
        print(f"Median response time:     {np.median(arr):.2f} ms")
        print(f"95th percentile:          {np.percentile(arr, 95):.2f} ms")
        print(f"99th percentile:          {np.percentile(arr, 99):.2f} ms")
        print(f"Max response time:        {arr.max():.2f} ms")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--ip", default="16.148.92.77", help="EC2 public IP")
    parser.add_argument("--port", type=int, default=8080, help="Server port")
    parser.add_argument("--path", default="/albums", help="Request path")
    parser.add_argument("--duration", type=int, default=30, help="Test duration in seconds")
    parser.add_argument("--timeout", type=int, default=10, help="Per-request timeout in seconds")
    parser.add_argument("--sleep-ms", type=int, default=0, help="Sleep between requests (ms)")
    args = parser.parse_args()

    url = f"http://{args.ip}:{args.port}{args.path}"

    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    prefix = f"loadtest_{args.ip.replace('.', '-')}_{ts}"

    results = load_test(url, duration_seconds=args.duration, timeout_seconds=args.timeout, sleep_ms=args.sleep_ms)

    csv_path = f"{prefix}.csv"
    save_csv(results, csv_path)
    print(f"\nSaved raw data: {csv_path}")

    print_stats(results)
    plot_results(results, prefix)


if __name__ == "__main__":
    main()
