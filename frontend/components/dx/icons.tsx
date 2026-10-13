import type { SVGProps } from "react";

/**
 * The icon set of the DX interface, drawn on one 24-unit grid: a 1.6 stroke, square ends, mitred joins, geometric shapes,
 * no fills. They are a family: same weight, same corner, same optical size, so any two read as made by one hand.
 */
const P: Record<string, string> = {
  overview: "M3 3h8v9H3z M13 3h8v5h-8z M13 10h8v11h-8z M3 14h8v7H3z",
  projects: "M3 5h7l2 3h9v12H3z M3 12h18",
  kubernetes: "M12 2.5l8.5 4.7v9.6L12 21.5l-8.5-4.7V7.2z M12 7.5v9 M8 9.8l8 4.4 M16 9.8l-8 4.4",
  carbon: "M4 20C4 11 9 5 20 4c0 11-6 16-14 16z M4 20l9-9",
  reports: "M5 3h10l4 4v14H5z M15 3v4h4 M8 11h8 M8 14h8 M8 17h5",
  recommendations: "M12 2.5l6.5 4v7l-3 2.5V19h-7v-3L5.5 13.5v-7z M9.5 22h5",
  automation: "M3 4h8v6H3z M13 14h8v6h-8z M7 10v7h6 M17 14V9h-3",
  cicd: "M4 3h5v5H4z M4 16h5v5H4z M15 3h5v5h-5z M6.5 8v8 M17.5 8v3.5H6.5",
  connectors: "M9 3v5 M15 3v5 M6 8h12v6l-4 4h-4l-4-4z M12 18v3",
  alerts: "M12 3l6.5 4v6l2 3.5h-17l2-3.5V7z M9.5 20h5",
  settings: "M3 7h9 M16 7h5 M3 17h4 M11 17h10 M12 4h4v6h-4z M7 14h4v6H7z",
  search: "M4 4h11v11H4z M15 15l5.5 5.5",
  chevronDown: "M5 9l7 7 7-7",
  chevronRight: "M9 5l7 7-7 7",
  chevronLeft: "M15 5l-7 7 7 7",
  collapse: "M12 6l-6 6 6 6 M20 6l-6 6 6 6",
  expand: "M12 6l6 6-6 6 M4 6l6 6-6 6",
  close: "M5 5l14 14 M19 5L5 19",
  plus: "M12 4v16 M4 12h16",
  minus: "M4 12h16",
  check: "M4 12.5l5 5L20 6.5",
  info: "M4 4h16v16H4z M12 11v6 M12 7v1.2",
  cost: "M3 6h18v14H3z M3 10.5h18 M15 15h3",
  emissions: "M3 20V11l6 4v-4l6 4V6h3v14z M3 20h18",
  intensity: "M13.5 2L5 14h6l-1.5 8L19 10h-6.5z",
  potential: "M12 3v13 M6 11l6 6 6-6 M4 21h16",
  forecast: "M3 3v18h18 M7 16l4-5 3 3 6-8",
  budget: "M4 4h16v16H4z M4 14h16 M12 14v6",
  anomaly: "M2 12h5l3-8 4 16 3-8h5",
  scale: "M12 3v18 M5 21h14 M4 7h16 M4 7l-2.5 7h5z M20 7l-2.5 7h5z",
  trendUp: "M5 17l6-6 4 4 5-7 M14 8h6v6",
  trendDown: "M5 7l6 6 4-4 5 7 M14 16h6v-6",
  arrowUp: "M12 19V5 M6 11l6-6 6 6",
  arrowDown: "M12 5v14 M6 13l6 6 6-6",
  arrowRight: "M4 12h16 M14 6l6 6-6 6",
  cloud: "M6 18h12l3.5-3.5-2-4.5h-4L12.5 6 8 9.5l-4 1.5z",
  pin: "M12 21.5l-6-8V3.5h12v10z M10 7.5h4v4h-4z",
  calendar: "M3 5h18v16H3z M3 10h18 M8 3v4 M16 3v4",
  filter: "M3 4h18l-7 8.5V20l-4 1.5v-9z",
  download: "M12 3v12 M6.5 10l5.5 5.5 5.5-5.5 M4 21h16",
  external: "M14 3h7v7 M21 3l-10 10 M10 5H4v15h15v-6",
  user: "M4 21v-4l4-3h8l4 3v4z M9 4h6v6H9z",
  globe: "M3 3h18v18H3z M3 12h18 M12 3v18 M3 7.5h18 M3 16.5h18",
  logout: "M10 4H4v16h6 M14 8l5 4-5 4 M19 12H9",
  menu: "M3 6h18 M3 12h18 M3 18h18",
  refresh: "M20 11a8 8 0 0 0-14.5-3.5 M4 3.5v4h4 M4 13a8 8 0 0 0 14.5 3.5 M20 20.5v-4h-4",
  more: "M4 10h4v4H4z M10 10h4v4h-4z M16 10h4v4h-4z",
  layers: "M12 3l9 5-9 5-9-5z M3 12.5l9 5 9-5 M3 17l9 5 9-5",
  compute: "M7 7h10v10H7z M10 3v4 M14 3v4 M10 17v4 M14 17v4 M3 10h4 M3 14h4 M17 10h4 M17 14h4",
  storage: "M3 5h18v6H3z M3 13h18v6H3z M7 8h2 M7 16h2",
  database: "M5 4h14v4H5z M5 10h14v4H5z M5 16h14v4H5z",
  networking: "M10 3h4v4h-4z M3 17h4v4H3z M17 17h4v4h-4z M12 7v5 M5 17v-5h14v5",
  other: "M5 5h6v6H5z M13 5h6v6h-6z M5 13h6v6H5z M13 13h6v6h-6z",
  sliders: "M4 6h16 M4 12h16 M4 18h16 M8 3v6 M16 9v6 M10 15v6",
  eye: "M2 12l4-5h12l4 5-4 5H6z M10 10h4v4h-4z",
  sun: "M12 7h0 M8 8h8v8H8z M12 2v3 M12 19v3 M2 12h3 M19 12h3",
  bolt: "M13.5 2L5 14h6l-1.5 8L19 10h-6.5z",
  shield: "M12 2.5l8 3v7c0 5-3.5 8-8 9-4.5-1-8-4-8-9v-7z",
  leaf: "M4 20C4 11 9 5 20 4c0 11-6 16-14 16z",
  list: "M8 6h13 M8 12h13 M8 18h13 M3 5h3v3H3z M3 11h3v3H3z M3 17h3v3H3z",
  clock: "M3 3h18v18H3z M12 7v5.5l4 2",
  dev: "M8 7l-5 5 5 5 M16 7l5 5-5 5 M14 4l-4 16",
};

export type DxIconName = keyof typeof P;

export function Ico({ name, size = 20, strokeWidth = 1.6, ...rest }: { name: DxIconName; size?: number; strokeWidth?: number } & Omit<SVGProps<SVGSVGElement>, "name">) {
  return (
    <svg viewBox="0 0 24 24" width={size} height={size} fill="none" stroke="currentColor" strokeWidth={strokeWidth} strokeLinecap="square" strokeLinejoin="miter" aria-hidden focusable="false" {...rest}>
      <path d={P[name]} />
    </svg>
  );
}

/** A cloud as a flat square with its initials in its own colour: one drawn mark per cloud, never a logo that clashes. */
export function CloudMark({ id, color }: { id: string; color: string }) {
  const letters: Record<string, string> = { aws: "A", azure: "Az", gcp: "G", digitalocean: "Do", kubernetes: "K8", saas: "S" };
  return <span className="dx-cloud" style={{ background: color }} aria-hidden>{letters[id] ?? id.slice(0, 1).toUpperCase()}</span>;
}
