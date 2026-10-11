/** Where a latency figure comes from, in words: the console never shows a number without its basis. */
type T = (key: string, params?: Record<string, string | number>) => string;

export function latencyBasisLabel(t: T, basis: string | undefined, errorPct?: number | string): string {
  switch (basis) {
    case "measured_azure": return t("lat.basis.measured_azure");
    case "measured_aws": return t("lat.basis.measured_aws");
    case "estimated": return t("lat.basis.estimated", { pct: Number(errorPct) || 0 });
    default: return t("lat.basis.unknown");
  }
}

export const isMeasured = (basis: string | undefined) => basis === "measured_azure" || basis === "measured_aws";
