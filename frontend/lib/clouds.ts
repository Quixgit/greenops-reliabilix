// What a cloud (or a vendor) looks like everywhere on the Overview: one name, one soft colour, so AWS is the same amber in the
// cost band, the stacked chart, the bars and the table. A provider the platform has no mark for still gets a stable colour.
export interface CloudMeta { id: string; name: string; /** the colour of its bars and areas */ color: string; /** a tint for a chip or a tile background */ soft: string; /** readable text on the tint */ ink: string }

const KNOWN: Record<string, CloudMeta> = {
  aws: { id: "aws", name: "AWS", color: "#f0b866", soft: "#fdf3df", ink: "#8a5a12" },
  azure: { id: "azure", name: "Azure", color: "#7aa7ea", soft: "#e8f0fd", ink: "#2f5fa8" },
  gcp: { id: "gcp", name: "Google Cloud", color: "#ee9aa6", soft: "#fdecee", ink: "#9a3a4a" },
  digitalocean: { id: "digitalocean", name: "DigitalOcean", color: "#6cc4e0", soft: "#e3f5fb", ink: "#1f6f8c" },
  kubernetes: { id: "kubernetes", name: "Kubernetes", color: "#9b8fe0", soft: "#eeebfb", ink: "#4f43a6" },
  saas: { id: "saas", name: "SaaS", color: "#b7a6d9", soft: "#f2eefa", ink: "#5e4a8c" },
};
const ALIAS: Record<string, string> = { do: "digitalocean", "digital ocean": "digitalocean", digital_ocean: "digitalocean", k8s: "kubernetes", google: "gcp", "google cloud": "gcp", "google-cloud": "gcp", amazon: "aws", microsoft: "azure" };
const SPARE = ["#8bd0b4", "#e7b48f", "#a9c3e8", "#d6a3c8", "#c4cf8b"];

export function cloudId(provider: string): string {
  const k = provider.trim().toLowerCase();
  return ALIAS[k] ?? k;
}

export function cloudMeta(provider: string): CloudMeta {
  const id = cloudId(provider);
  if (KNOWN[id]) return KNOWN[id];
  let h = 0;
  for (const ch of id) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  const color = SPARE[h % SPARE.length];
  return { id, name: provider || "—", color, soft: "#f3f5f9", ink: "#475569" };
}
