export type PlannedSection = {
  title: string;
  summary: string;
  /** What exists in the console today that covers part of this. */
  today: { label: string; href: string }[];
  /** What the section will hold; the order is the build order. */
  scope: string[];
  /** Where it sits in the roadmap (docs/product/02-platform-admin.md). */
  stage: string;
};

/** Sections that are in the menu but not built. Each says honestly what is missing and what covers it meanwhile. */
export const PLANNED: Record<string, PlannedSection> = {
  aggregation: {
    title: "Aggregation & policies",
    summary: "Which source wins for which metric and region, what happens when it is stale, and where each value came from.",
    today: [{ label: "Data sources registry", href: "/operator/data-sources" }, { label: "Platform services", href: "/operator/data-sources" }],
    scope: ["Primary and fallback source per metric and region", "Freshness and completeness thresholds before a source is used",
      "Average and marginal intensity are stored and shown separately, never averaged together", "Unit, interval and currency normalization rules",
      "Provenance of every aggregated value: source, time received, time measured, licence", "Recalculation after a methodology change, with old results kept"],
    stage: "Phase 2 (Citable carbon coefficients, second grid source)",
  },
};
