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
export const PLANNED: Record<string, PlannedSection> = {};
