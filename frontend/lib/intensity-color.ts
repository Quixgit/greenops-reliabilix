// Warm scale in the DX palette, clean -> dirty: sage, butter, peach, brick. Thresholds are the grid-intensity bands used in reports.
export const STOPS: [number, [number, number, number]][] = [
  [0, [118, 164, 128]], [150, [186, 202, 140]], [300, [230, 205, 130]], [450, [226, 164, 120]], [600, [190, 100, 92]],
];
export const NEUTRAL = "#ddd6c8";

export function intensityColor(g: number): string {
  const v = Math.max(0, Math.min(600, g));
  for (let i = 1; i < STOPS.length; i++) {
    if (v <= STOPS[i][0]) {
      const [a, ca] = STOPS[i - 1], [b, cb] = STOPS[i], k = (v - a) / (b - a);
      return `rgb(${ca.map((c, j) => Math.round(c + (cb[j] - c) * k)).join(",")})`;
    }
  }
  return `rgb(${STOPS[STOPS.length - 1][1].join(",")})`;
}
