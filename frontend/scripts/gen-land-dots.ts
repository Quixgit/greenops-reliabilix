// Generates lib/land-dots.json: the dot-matrix land of the region map, computed once here instead of in every visitor's
// browser. Run (Node 20): npx esbuild scripts/gen-land-dots.ts --bundle --platform=node --format=esm --outfile=/tmp/gen.mjs
// and then node /tmp/gen.mjs from the frontend directory.
import { writeFileSync } from "node:fs";
import { feature } from "topojson-client";
import type { Topology } from "topojson-specification";
import world from "world-atlas/countries-110m.json";
import { cleanWorld } from "../lib/geo";

type Ring = [number, number][];
type Shape = { name: string; polys: Ring[][]; box: [number, number, number, number] };

const inRing = (x: number, y: number, ring: Ring) => {
  let inside = false;
  for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
    const [xi, yi] = ring[i], [xj, yj] = ring[j];
    if (yi > y !== yj > y && x < ((xj - xi) * (y - yi)) / (yj - yi) + xi) inside = !inside;
  }
  return inside;
};

const topo = world as unknown as Topology;
const raw = feature(topo, topo.objects.countries as never) as unknown as Parameters<typeof cleanWorld>[0];
const shapes: Shape[] = cleanWorld(raw).features.map((f) => {
  const geom = f.geometry as { type: string; coordinates: unknown };
  const polys = (geom.type === "Polygon" ? [geom.coordinates] : geom.coordinates) as Ring[][];
  let x0 = 180, y0 = 90, x1 = -180, y1 = -90;
  for (const p of polys) for (const [x, y] of p[0]) { x0 = Math.min(x0, x); x1 = Math.max(x1, x); y0 = Math.min(y0, y); y1 = Math.max(y1, y); }
  return { name: String(f.properties?.name ?? ""), polys, box: [x0, y0, x1, y1] };
});

const step = 2.4;
const found: { x: number; y: number; country: string }[] = [];
for (let lat = 79; lat > -58; lat -= step) {
  for (let lon = -179; lon < 180; lon += step) {
    const s = shapes.find((sh) => lon >= sh.box[0] && lon <= sh.box[2] && lat >= sh.box[1] && lat <= sh.box[3]
      && sh.polys.some((p) => inRing(lon, lat, p[0]) && !p.slice(1).some((h) => inRing(lon, lat, h))));
    if (s) found.push({ x: lon + 180, y: 80 - lat, country: s.name });
  }
}
const countries = [...new Set(found.map((d) => d.country))].sort();
const idx = new Map(countries.map((c, i) => [c, i]));
// x and y in tenths of a degree, then the country index: a flat array of triples.
const flat: number[] = [];
for (const d of found) flat.push(Math.round(d.x * 10), Math.round(d.y * 10), idx.get(d.country)!);
writeFileSync("lib/land-dots.json", JSON.stringify({ countries, dots: flat }));
console.log(`${found.length} dots, ${countries.length} countries`);
