import json, re
from collections import Counter

# 1) load reducer output
with open("final.json", "r", encoding="utf-8") as f:
    reducer = json.load(f)

print("final.json keys:", reducer.keys())

counts = None
for k, v in reducer.items():
    if isinstance(v, dict):
        counts = v
        counts_key = k
        break
if counts is None:
    raise RuntimeError("Can't find word-count dict in final.json. Open final.json and check structure.")

print("Using counts field:", counts_key, "size:", len(counts))

# 2) ground truth from raw text (match mapper normalization)
with open("hamlet.txt", "r", encoding="utf-8", errors="ignore") as f:
    text = f.read().lower()

words = re.findall(r"[a-z0-9]+", text)
gt = Counter(words)

# 3) compare
# reducer counts might be strings -> ints; normalize
counts_norm = {w: int(c) for w, c in counts.items()}

missing = []
diff = []
for w, c in gt.items():
    if w not in counts_norm:
        missing.append(w)
    elif counts_norm[w] != c:
        diff.append((w, counts_norm[w], c))

extra = [w for w in counts_norm.keys() if w not in gt]

print("GT unique words:", len(gt))
print("Reducer unique words:", len(counts_norm))
print("Missing in reducer:", len(missing))
print("Extra in reducer:", len(extra))
print("Different counts:", len(diff))

# show a few examples
print("\nExamples of different counts (up to 20):")
for w, got, exp in diff[:20]:
    print(w, "reducer=", got, "gt=", exp)

# quick sanity check: top-20 words
print("\nTop-20 GT words:")
for w, c in gt.most_common(20):
    print(w, c)
