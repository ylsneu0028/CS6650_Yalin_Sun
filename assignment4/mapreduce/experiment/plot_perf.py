import pandas as pd
import matplotlib.pyplot as plt

df = pd.read_csv("perf.csv")

# 平均值
g = df.groupby("mappers", as_index=False).agg(
    split_s=("split_s", "mean"),
    map_s=("map_s", "mean"),
    reduce_s=("reduce_s", "mean"),
    total_s=("total_s", "mean"),
)

# 图1：总耗时
plt.figure()
plt.plot(g["mappers"], g["total_s"], marker="o")
plt.xlabel("Number of Mappers")
plt.ylabel("Avg End-to-End Time (s)")
plt.title("End-to-End Time vs Parallelism")
plt.savefig("total_time.png", dpi=200)

# 图2：分解耗时
plt.figure()
plt.plot(g["mappers"], g["split_s"], marker="o", label="split")
plt.plot(g["mappers"], g["map_s"], marker="o", label="map")
plt.plot(g["mappers"], g["reduce_s"], marker="o", label="reduce")
plt.xlabel("Number of Mappers")
plt.ylabel("Avg Time (s)")
plt.title("Time Breakdown vs Parallelism")
plt.legend()
plt.savefig("breakdown.png", dpi=200)

print(g)
print("Saved: total_time.png, breakdown.png")
