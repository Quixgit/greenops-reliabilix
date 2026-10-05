import { z } from "zod";
import raw from "./overview.json";

// dataSchema: all editable copy of the Overview screen lives in content/overview.json and is
// validated here at build/start, so it can be changed without touching components.
export const overviewContentSchema = z.object({
  pageTitle: z.string(),
  pageSubtitle: z.string(),
  sidebarPromo: z.object({ title: z.string(), text: z.string() }),
  promoBanner: z.object({ title: z.string(), text: z.string(), cta: z.string(), imageUrl: z.string() }),
  recommendationsEmpty: z.string(),
  tagline: z.string(),
  empty: z.object({ noCloud: z.string(), awaitingSync: z.string(), noUsage: z.string(), noActivity: z.string() }),
});

export type OverviewContent = z.infer<typeof overviewContentSchema>;
export const content: OverviewContent = overviewContentSchema.parse(raw);
