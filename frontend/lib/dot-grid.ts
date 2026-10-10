import data from "./land-dots.json";

/** Map space: x = longitude + 180 (0..360), y = 80 - latitude (0..138). One unit is one degree (equirectangular). */
export const MAP_W = 360;
export const MAP_H = 138;
export const project = (lat: number, lon: number): [number, number] => [lon + 180, 80 - lat];

export type Dot = { x: number; y: number; country: string };

let cache: Dot[] | null = null;

/**
 * Land on a regular grid (a dot-matrix world), each dot knowing its country so it can be tinted by that country's grid.
 * The points are precomputed (scripts/gen-land-dots.ts): nothing is calculated in the browser.
 */
export function landDots(): Dot[] {
  if (cache) return cache;
  const out: Dot[] = [];
  const d = data.dots;
  for (let i = 0; i < d.length; i += 3) out.push({ x: d[i] / 10, y: d[i + 1] / 10, country: data.countries[d[i + 2]] });
  return (cache = out);
}
