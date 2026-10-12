import { IconSrcElectricityMaps, IconSrcEmber, IconSrcEia, IconSrcEnergyCharts, IconSrcIeso, IconSrcHydroQuebec, IconSrcEntsoe, IconSrcOns, IconSrcUkGrid, IconSrcWattTime, IconAws, IconAzure, IconCpu, IconDatabase, IconGcp, IconKubernetes, IconMail, IconPlug, IconSend, IconUsers, type IconType } from "@/components/ui/icons";
import { cn } from "@/lib/utils";

const MARKS: Record<string, { icon: IconType; tone: string; own?: boolean }> = {
  aws: { icon: IconAws, tone: "", own: true },
  azure: { icon: IconAzure, tone: "", own: true },
  gcp: { icon: IconGcp, tone: "", own: true },
  kubernetes: { icon: IconKubernetes, tone: "", own: true },
  email: { icon: IconMail, tone: "text-slate-500" },
  slack: { icon: IconSend, tone: "text-[#8a78d8]" },
  webhook: { icon: IconPlug, tone: "text-[#5b8fdc]" },
  teams: { icon: IconUsers, tone: "text-[#8a78d8]" },
  prometheus: { icon: IconCpu, tone: "text-[#d9a441]" },
  datadog: { icon: IconDatabase, tone: "text-[#8a78d8]" },
  snowflake: { icon: IconDatabase, tone: "text-[#5b8fdc]" },
  "custom-csv": { icon: IconDatabase, tone: "text-slate-500" },
  "electricity-maps": { icon: IconSrcElectricityMaps, tone: "", own: true },
  electricitymaps: { icon: IconSrcElectricityMaps, tone: "", own: true },
  energy_charts: { icon: IconSrcEnergyCharts, tone: "", own: true },
  ieso: { icon: IconSrcIeso, tone: "", own: true },
  hydroquebec: { icon: IconSrcHydroQuebec, tone: "", own: true },
  entsoe: { icon: IconSrcEntsoe, tone: "", own: true },
  ons: { icon: IconSrcOns, tone: "", own: true },
  eia: { icon: IconSrcEia, tone: "", own: true },
  ember: { icon: IconSrcEmber, tone: "", own: true },
  watttime: { icon: IconSrcWattTime, tone: "", own: true },
  uk_carbon_intensity: { icon: IconSrcUkGrid, tone: "", own: true },
};

/** The drawn mark of a connector: bare, no tile or frame, the same weight for every source so mixed lists read evenly. */
export function ProviderMark({ id, size = "md", dim = false }: { id: string; size?: "sm" | "md" | "lg"; dim?: boolean }) {
  const m = MARKS[id] ?? { icon: IconDatabase, tone: "text-slate-400" };
  const Icon = m.icon;
  return <Icon className={cn(size === "sm" ? "size-8" : size === "lg" ? "size-12" : "size-10", "shrink-0 transition-transform duration-300", m.tone, dim && "opacity-45 saturate-50")} strokeWidth={1.4} />;
}
