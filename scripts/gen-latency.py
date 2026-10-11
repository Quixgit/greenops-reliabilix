#!/usr/bin/env python3
"""Refreshes the measured region-to-region latency tables the platform ships with.

Sources (both public, no key):
  - Azure: Microsoft's "Azure network round-trip latency statistics" (learn.microsoft.com), measured by Microsoft between its regions,
    in ms, one table per group of destination regions.
  - AWS: CloudPing.co (open source, github.com/mda590/cloudping.co): TCP latency between every pair of AWS regions, measured from
    Lambda functions in each region every few hours. We take the median (p50) over one year.
GCP publishes no such table: its regions, and any pair of regions of different clouds, are estimated from distance with a line fitted
to the measured pairs (fit, and the error of the fit, are stored so the console can say how far to trust it).

Writes backend/internal/platform/geo/latency.json and a copy at frontend/lib/latency.json. Run it, review the diff, commit.
"""
import json, math, re, sys, urllib.request, datetime, pathlib

ROOT = pathlib.Path(__file__).resolve().parent.parent
AZURE = "https://learn.microsoft.com/en-us/azure/networking/azure-network-latency"
AWS = "https://www.cloudping.co/api/latencies?percentile=p_50&timeframe=1Y"


def get(url):
    req = urllib.request.Request(url, headers={"User-Agent": "reliabilix-gen-latency"})
    return urllib.request.urlopen(req, timeout=60).read().decode("utf-8")


def azure():
    html = get(AZURE)
    out = {}
    for t in re.findall(r"<table.*?</table>", html, re.S):
        rows = [[re.sub(r"<[^>]+>", "", c).strip() for c in re.findall(r"<t[hd][^>]*>(.*?)</t[hd]>", r, re.S)] for r in re.findall(r"<tr.*?</tr>", t, re.S)]
        head = rows[0][1:]
        for r in rows[1:]:
            src = r[0].lower().replace(" ", "")
            for dst, v in zip(head, r[1:]):
                if v.isdigit():
                    out.setdefault(src, {})[dst.lower().replace(" ", "")] = int(v)
    updated = re.search(r'updated_at" content="([^"]+)"', html)
    return out, (updated.group(1)[:10] if updated else "")


def aws():
    d = json.loads(get(AWS))["data"]
    return {a: {b: round(v) for b, v in row.items() if isinstance(v, (int, float))} for a, row in d.items()}


def places():
    src = (ROOT / "backend/internal/platform/geo/regions.go").read_text()
    return {m.group(1): (float(m.group(2)), float(m.group(3))) for m in re.finditer(r'"([a-z0-9-]+)":\s*\{"[^"]*", "[^"]*", "[A-Z]{2}", (-?[\d.]+), (-?[\d.]+)\}', src)}


def km(p, q):
    r = math.pi / 180
    dl, dn = (q[0] - p[0]) * r, (q[1] - p[1]) * r
    h = math.sin(dl / 2) ** 2 + math.cos(p[0] * r) * math.cos(q[0] * r) * math.sin(dn / 2) ** 2
    return 2 * 6371 * math.asin(math.sqrt(h))


def main():
    az, az_date = azure()
    aw = aws()
    pl = places()
    az = {a: {b: v for b, v in row.items() if b in pl} for a, row in az.items() if a in pl}
    aw = {a: {b: v for b, v in row.items() if b in pl} for a, row in aw.items() if a in pl}
    pts = [(km(pl[a], pl[b]), v) for tab in (az, aw) for a, row in tab.items() for b, v in row.items() if a != b and v > 3]
    n = len(pts)
    # least squares on the RELATIVE error (weights 1/y^2): a short hop must not be as wrong as a long one
    w = [1 / y ** 2 for _, y in pts]
    sw = sum(w)
    mx = sum(wi * x for wi, (x, _) in zip(w, pts)) / sw
    my = sum(wi * y for wi, (_, y) in zip(w, pts)) / sw
    slope = sum(wi * (x - mx) * (y - my) for wi, (x, y) in zip(w, pts)) / sum(wi * (x - mx) ** 2 for wi, (x, _) in zip(w, pts))
    icpt = my - slope * mx
    rel = sorted(abs(icpt + slope * x - y) / y for x, y in pts)
    fit = {"intercept_ms": round(icpt, 2), "per_km_ms": round(slope, 5), "pairs": n, "median_error_pct": round(rel[n // 2] * 100), "p80_error_pct": round(rel[int(n * .8)] * 100)}
    out = {
        "azure": {"source": AZURE, "published": az_date, "note": "Measured by Microsoft between its regions, round trip, ms", "ms": az},
        "aws": {"source": "https://www.cloudping.co", "fetched": datetime.date.today().isoformat(), "note": "TCP round trip between AWS regions, median over one year, ms", "ms": aw},
        "fit": fit,
    }
    text = json.dumps(out, separators=(",", ":"), sort_keys=True)
    for p in ("backend/internal/platform/geo/latency.json", "frontend/lib/latency.json"):
        (ROOT / p).write_text(text + "\n")
    print(f"azure {sum(len(r) for r in az.values())} pairs ({len(az)} regions), aws {sum(len(r) for r in aw.values())} ({len(aw)}), fit {fit}")


if __name__ == "__main__":
    sys.exit(main())
