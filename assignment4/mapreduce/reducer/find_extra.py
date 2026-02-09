import json, re
from collections import Counter

with open("final.json","r",encoding="utf-8") as f:
    reducer = json.load(f)
counts = reducer["counts"]

with open("hamlet.txt","r",encoding="utf-8",errors="ignore") as f:
    text = f.read().lower()
gt = Counter(re.findall(r"[a-z]+", text))

extra = sorted([w for w in counts.keys() if w not in gt])
print("extra:", extra)
