import { z } from "zod";

// Response contracts of the Go API (see backend/api/openapi.yaml). Parsing at the boundary means a
// backend change shows up as a clear error here instead of a broken chart.
const totals = z.object({
  energy_kwh: z.number(),
  carbon_kg_co2e: z.number(),
  sci_score: z.number().nullable(), // null until the customer reports functional units: never guessed
  carbon_intensity_g_per_kwh: z.number(),
});

export const carbonSummary = totals.extend({ has_data: z.boolean(), previous: totals, comparable: z.boolean() });
export type CarbonSummary = z.infer<typeof carbonSummary>;

export const finopsSummary = z.object({
  has_data: z.boolean(),
  total_cost: z.number(),
  previous_total_cost: z.number(),
  previous_comparable: z.boolean(),
  currency: z.string(),
  by_service: z.array(z.object({ service: z.string(), cost: z.number() })),
});
export type FinopsSummary = z.infer<typeof finopsSummary>;

const items = <T extends z.ZodTypeAny>(t: T) => z.object({ items: z.array(t) }).transform((o) => o.items);

export const trend = items(z.object({ day: z.string(), cost: z.number(), carbon_kg_co2e: z.number() }));
export type TrendPoint = z.infer<typeof trend>[number];

export const providers = items(z.object({ provider: z.string(), carbon_kg_co2e: z.number(), share_pct: z.number() }));
export type ProviderShare = z.infer<typeof providers>[number];

export const services = items(z.object({
  service: z.string(), category: z.string(), cost: z.number(), carbon_kg_co2e: z.number(), trend: z.array(z.number()),
}));
export type ServiceRow = z.infer<typeof services>[number];

export const regions = items(z.object({
  region: z.string(), g_per_kwh: z.number(), change_pct: z.number().nullable(), at: z.string(),
}));
export type RegionIntensity = z.infer<typeof regions>[number];

export const activity = items(z.object({ id: z.number(), action: z.string(), target: z.string(), at: z.string() }));
export type Activity = z.infer<typeof activity>[number];

export const project = z.object({
  id: z.string(), name: z.string(), functional_unit: z.string(), created_at: z.string(),
});
export const projects = items(project);
export type Project = z.infer<typeof project>;

export const connection = z.object({
  id: z.string(), project_id: z.string(), provider: z.string(), account_ref: z.string(),
  sync_status: z.string(), last_sync_at: z.string().nullable(),
});
export const connections = items(connection);

export const setup = z.object({
  external_id: z.string(),
  platform_account_id: z.string().optional(),
  trust_policy: z.record(z.string(), z.unknown()).optional(),
  permissions_policy: z.record(z.string(), z.unknown()),
  required_permissions: z.array(z.string()),
});
export const connectResponse = z.object({ connection: connection, setup });
export type ConnectResponse = z.infer<typeof connectResponse>;
export const tenantRef = z.object({ tenant_id: z.string() });
export type Connection = z.infer<typeof connection>;

export const recommendation = z.object({
  id: z.string(),
  project_id: z.string(),
  type: z.string(),
  title: z.string(),
  provider: z.string(),
  service_name: z.string(),
  current_region: z.string(),
  recommended_region: z.string(),
  estimated_carbon_reduction_pct: z.number(),
  carbon_reduction_kg_month: z.number(),
  estimated_cost_impact: z.number().nullable(), // null = not estimated: never shown as a saving
  cost_basis: z.string(),
  confidence: z.number(),
  compliance_check: z.object({ residency: z.string(), allowed_regions: z.array(z.string()).optional() }).passthrough(),
  status: z.enum(["open", "approved", "applied", "dismissed"]),
});
export type Recommendation = z.infer<typeof recommendation>;
export const recommendations = items(recommendation);

export const report = z.object({
  id: z.string(),
  project_id: z.string().nullable(),
  kind: z.enum(["carbon", "sci", "finops", "sustainability"]),
  format: z.enum(["csv", "json", "pdf"]),
  period_start: z.string(),
  period_end: z.string(),
  status: z.enum(["pending", "ready", "failed"]),
  error: z.string().nullable().optional(),
  created_at: z.string(),
});
export type Report = z.infer<typeof report>;
export const reports = items(report);
