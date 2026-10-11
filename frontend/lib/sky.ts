/**
 * How dark the sign-in sky is at a given local time. Day lasts from about 07:30 to 17:30; sunrise (05:00-07:30) and sunset
 * (17:30-20:30) fade smoothly, night is the rest. `glow` is the warm light on the horizon while the sun is near it.
 * It is the visitor's own clock, so "it is night where they are" without asking for their location.
 */
export type Sky = { night: number; glow: number; morning: boolean };

const smooth = (a: number, b: number, x: number) => {
  const t = Math.min(1, Math.max(0, (x - a) / (b - a)));
  return t * t * (3 - 2 * t);
};

export function skyAt(hour: number): Sky {
  const h = ((hour % 24) + 24) % 24;
  const morning = h < 12;
  const day = morning ? smooth(5, 7.5, h) : 1 - smooth(17.5, 20.5, h);
  const glow = morning ? Math.exp(-(((h - 6.2) / 1.0) ** 2)) : Math.exp(-(((h - 19.0) / 1.1) ** 2));
  return { night: 1 - day, glow, morning };
}

/** `?sky=21:30` previews a time of day; anything else is the local clock. */
export function localHour(now: Date = new Date(), override?: string | null): number {
  const m = override?.match(/^(\d{1,2})(?::(\d{2}))?$/);
  if (m && +m[1] < 24) return +m[1] + (m[2] ? Math.min(59, +m[2]) / 60 : 0);
  return now.getHours() + now.getMinutes() / 60;
}
