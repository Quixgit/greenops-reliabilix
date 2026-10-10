// Pastel scale, clean -> dirty: mint, butter, peach, rose. Thresholds are the grid-intensity bands used in reports.
export const STOPS: [number, [number, number, number]][] = [
  [0, [143, 227, 192]], [150, [214, 233, 148]], [300, [250, 214, 140]], [450, [248, 175, 140]], [600, [238, 140, 160]],
];
export const NEUTRAL = "#e4eaf3";

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
