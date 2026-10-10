/**
 * Natural Earth country shapes contain rings that cross the antimeridian (Russia's Chukotka, Fiji). Drawn on a flat
 * lon/lat map the ring jumps from +180 to -180 and paints a stripe across the whole map. These helpers cut such rings
 * at +-180 so each part is drawn where it belongs.
 */
type Pt = [number, number];
type Ring = Pt[];

const crosses = (ring: Ring) => ring.some((p, i) => i > 0 && Math.abs(p[0] - ring[i - 1][0]) > 180);

/** Makes longitudes continuous: whenever the next point jumps by more than 180 degrees, shift it by a full turn. */
function unwrap(ring: Ring): Ring {
  const out: Ring = [];
  let shift = 0;
  ring.forEach((p, i) => {
    if (i > 0) {
      const d = p[0] + shift - out[i - 1][0];
      if (d > 180) shift -= 360;
      else if (d < -180) shift += 360;
    }
    out.push([p[0] + shift, p[1]]);
  });
  return out;
}

/** Sutherland-Hodgman clip of a ring against the vertical strip lo <= x <= hi. */
function clipX(ring: Ring, lo: number, hi: number): Ring {
  const clipEdge = (pts: Ring, inside: (p: Pt) => boolean, edgeX: number): Ring => {
    const res: Ring = [];
    pts.forEach((cur, i) => {
      const prev = pts[(i + pts.length - 1) % pts.length];
      const ci = inside(cur), pi = inside(prev);
      if (ci !== pi) {
        const t = (edgeX - prev[0]) / (cur[0] - prev[0]);
        res.push([edgeX, prev[1] + t * (cur[1] - prev[1])]);
      }
      if (ci) res.push(cur);
    });
    return res;
  };
  return clipEdge(clipEdge(ring, (p) => p[0] >= lo, lo), (p) => p[0] <= hi, hi);
}

/** Splits one ring into the parts that lie inside [-180, 180] after unwrapping. Rings that do not cross are returned as is. */
export function splitRing(ring: Ring): Ring[] {
  if (!crosses(ring)) return [ring];
  const u = unwrap(ring);
  const parts: Ring[] = [];
  for (const turn of [-1, 0, 1]) { // the shape may extend past +180 or -180 after unwrapping
    const lo = -180 + 360 * turn, hi = 180 + 360 * turn;
    const piece = clipX(u, lo, hi).map((p) => [p[0] - 360 * turn, p[1]] as Pt);
    if (piece.length >= 3) parts.push(piece);
  }
  return parts;
}

type Geometry = { type: "Polygon"; coordinates: Ring[] } | { type: "MultiPolygon"; coordinates: Ring[][] } | { type: string; coordinates?: unknown };
type Feature = { type: "Feature"; properties?: { name?: string }; geometry: Geometry };

/** Fixes antimeridian-crossing rings and drops Antarctica (it only wastes map space and has no cloud regions). */
export function cleanWorld<T extends { features: Feature[] }>(fc: T, drop: string[] = ["Antarctica"]): T {
  const features = fc.features
    .filter((f) => !drop.includes(f.properties?.name ?? ""))
    .map((f): Feature => {
      const g = f.geometry;
      if (g.type === "Polygon") {
        const rings = (g as { coordinates: Ring[] }).coordinates;
        if (!crosses(rings[0])) return f;
        const parts = splitRing(rings[0]); // holes of a ring that crosses the antimeridian are dropped (none in the 110m data)
        return { ...f, geometry: { type: "MultiPolygon", coordinates: parts.map((r) => [r]) } };
      }
      if (g.type === "MultiPolygon") {
        const polys = (g as { coordinates: Ring[][] }).coordinates;
        const out: Ring[][] = [];
        for (const poly of polys) {
          if (!crosses(poly[0])) out.push(poly);
          else for (const part of splitRing(poly[0])) out.push([part]);
        }
        return { ...f, geometry: { type: "MultiPolygon", coordinates: out } };
      }
      return f;
    });
  return { ...fc, features };
}
