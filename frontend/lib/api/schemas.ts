import { z } from "zod";

// Response contracts of the Go API (see backend/api/openapi.yaml). Parsing at the boundary means a
// backend change shows up as a clear error here instead of a broken chart.
const totals = z.object({
  energy_kwh: z.number(),
  carbon_kg_co2e: z.number(),
  sci_score: z.number().nullable(), // null until the customer reports functional units: never guessed
  carbon_intensity_g_per_kwh: z.number(),
});

// The methodology travels with the figures so the UI can say what they are and are not (never a bare number).
export const methodologyInfo = z.object({
  version: z.string(),
  status: z.string(),
  provenance: z.string(),
  embodied_carbon_included: z.boolean(),
  caveats: z.array(z.string()),
});
export type MethodologyInfo = z.infer<typeof methodologyInfo>;

export const carbonSummary = totals.extend({ has_data: z.boolean(), previous: totals, comparable: z.boolean(), methodology: methodologyInfo });
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

// Measured result of an applied recommendation. Kept apart from the forecast fields on purpose.
export const impactVerification = z.object({
  outcome: z.enum(["realized", "partial", "not_realized", "inconclusive"]),
  baseline_days: z.number(),
  after_days: z.number(),
  measured_carbon_kg_month: z.number().nullable(),
  measured_cost_month: z.number().nullable(),
  expected_carbon_kg_month: z.number(),
  expected_cost_month: z.number().nullable(),
  realization_pct: z.number().nullable(),
  note: z.string(),
  verified_at: z.string(),
});
export type ImpactVerification = z.infer<typeof impactVerification>;

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
  status: z.enum(["open", "approved", "applied", "verified", "dismissed"]),
  verification: impactVerification.nullish(),
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

// ---- Settings: notifications, members, API keys (ADR-0019) ----------------------------------------------------
export const severity = z.enum(["info", "warning", "critical"]);
export type Severity = z.infer<typeof severity>;

export const notificationCatalog = z.object({
  channels: z.array(z.string()),
  configured: z.boolean(),
  stubbed: z.array(z.string()), // channels running on a development stand-in
  event_types: z.array(z.object({ type: z.string(), description: z.string(), default_severity: severity })),
});
export type NotificationCatalog = z.infer<typeof notificationCatalog>;

export const destination = z.object({
  id: z.string(), kind: z.string(), name: z.string(), target_hint: z.string(), event_types: z.array(z.string()),
  min_severity: severity, enabled: z.boolean(), created_by: z.string(), created_at: z.string(),
});
export type Destination = z.infer<typeof destination>;
export const destinations = items(destination);
export const createdDestination = z.object({ destination, signing_secret: z.string().optional() });

export const deliveryStatus = z.enum(["pending", "sent", "failed", "skipped"]);
export type DeliveryStatus = z.infer<typeof deliveryStatus>;
export const delivery = z.object({
  id: z.string(), destination_id: z.string().nullable(), kind: z.string(), destination_name: z.string(), event_type: z.string(),
  severity, status: deliveryStatus, attempts: z.number(), next_attempt_at: z.string(), last_error: z.string().nullable(),
  created_at: z.string(), sent_at: z.string().nullable(),
});
export type Delivery = z.infer<typeof delivery>;
export const deliveries = items(delivery);
export const testQueued = z.object({ delivery_id: z.string() });

export const member = z.object({ id: z.string(), user_id: z.string(), email: z.string(), role: z.string(), created_at: z.string() });
export type Member = z.infer<typeof member>;
export const members = items(member);
export const invitation = z.object({
  id: z.string(), email: z.string(), role: z.string(), created_by: z.string(), created_at: z.string(), expires_at: z.string(), accepted_at: z.string().nullable(),
});
export const invitations = items(invitation);
export const invitationCreated = z.object({ invitation, token: z.string() });
export const apiKey = z.object({
  id: z.string(), name: z.string(), prefix: z.string(), role: z.string(), created_by: z.string(), created_at: z.string(),
  last_used_at: z.string().nullable(), revoked_at: z.string().nullable(),
});
export type ApiKey = z.infer<typeof apiKey>;
export const apiKeys = items(apiKey);
export const apiKeyCreated = z.object({ api_key: apiKey, key: z.string() });

// ---- Operator console (ADR-0020): metadata across organizations, never tenant data --------------------------
export const operatorMe = z.object({ subject: z.string(), email: z.string(), role: z.enum(["viewer", "admin"]) });
export type OperatorMe = z.infer<typeof operatorMe>;
export const operatorTenant = z.object({
  id: z.string(), name: z.string(), plan: z.string(), created_at: z.string(), members: z.number(), projects: z.number(),
  connections: z.number(), connections_in_error: z.number(), destinations: z.number(), failed_deliveries_7d: z.number(),
});
export type OperatorTenant = z.infer<typeof operatorTenant>;
export const operatorTenants = items(operatorTenant);
export const operatorDelivery = delivery.omit({ destination_id: true }).extend({
  tenant_id: z.string(), tenant_name: z.string(), id: z.string(), destination_id: z.string().nullable(),
});
export type OperatorDelivery = z.infer<typeof operatorDelivery>;
export const operatorDeliveries = items(operatorDelivery);
export const operatorStats = items(z.object({ kind: z.string(), status: z.string(), count: z.number() }));
export const operatorConnection = z.object({
  tenant_id: z.string(), tenant_name: z.string(), provider: z.string(), account_ref: z.string(), sync_status: z.string(),
  last_error: z.string().nullable(), last_sync_at: z.string().nullable(),
});
export const operatorConnections = items(operatorConnection);
export const integrationStatus = z.enum(["ready", "stub", "missing"]);
export type IntegrationStatus = z.infer<typeof integrationStatus>;
export const operatorSystem = z.object({
  version: z.string(), env: z.string(), expected_migration: z.number(),
  integrations: z.array(z.object({ name: z.string(), status: integrationStatus, note: z.string() })),
});
export type OperatorSystem = z.infer<typeof operatorSystem>;

export const errorStatus = z.enum(["open", "resolved", "ignored"]);
export type ErrorStatus = z.infer<typeof errorStatus>;
export const operatorError = z.object({
  id: z.string(), service: z.string(), component: z.string(), severity: z.enum(["warning", "error", "critical"]), message: z.string(),
  status: errorStatus, occurrences: z.number(), first_seen: z.string(), last_seen: z.string(), tenant_id: z.string().nullable(),
  tenant_name: z.string().nullable(), last_request_id: z.string().nullable(), handled_by: z.string().nullable(), handled_at: z.string().nullable(),
});
export type OperatorError = z.infer<typeof operatorError>;
export const operatorErrors = items(operatorError);
export const changed = z.record(z.string(), z.unknown());

// ---- Settings engine (ADR-0023): the form is generated from this description ------------------------------------
export const settingScope = z.enum(["platform", "tenant", "operator"]);
export const settingType = z.enum(["bool", "int", "float", "string", "enum", "list"]);
export type SettingType = z.infer<typeof settingType>;
export const settingSource = z.enum(["default", "platform", "tenant", "operator"]);
export type SettingSource = z.infer<typeof settingSource>;
export const settingItem = z.object({
  key: z.string(), group: z.string(), title: z.string(), description: z.string(), note: z.string().optional(),
  scope: settingScope, type: settingType, default: z.unknown(), value: z.unknown(),
  source: settingSource, locked: z.boolean(), editable: z.boolean(),
  unit: z.string().optional(), min: z.number().optional(), max: z.number().optional(),
  enum: z.array(z.string()).optional(), list_enum: z.array(z.string()).optional(),
  max_items: z.number().optional(), min_items: z.number().optional(), max_len: z.number().optional(), pattern: z.string().optional(),
  updated_by: z.string().nullable().optional(), updated_at: z.string().nullable().optional(),
});
export type SettingItem = z.infer<typeof settingItem>;
export const settingsView = z.object({
  groups: z.array(z.object({ id: z.string(), title: z.string(), description: z.string() })),
  items: z.array(settingItem),
});
export type SettingsView = z.infer<typeof settingsView>;
export const settingChange = z.object({
  at: z.string(), tenant_id: z.string().nullable(), key: z.string(), old_value: z.unknown(), new_value: z.unknown(), actor: z.string(), via: z.enum(["tenant", "operator"]),
});
export const settingChanges = items(settingChange);

export const anomaly = z.object({ day: z.string(), cost: z.number(), expected: z.number(), stddev: z.number() });
export type Anomaly = z.infer<typeof anomaly>;
export const anomalies = items(anomaly);
export const carbonTrend = items(z.object({ day: z.string(), carbon_kg_co2e: z.number() }));

export const methodologyVersions = items(z.object({ version: z.string(), status: z.string(), provenance: z.string(), active: z.boolean() }));
