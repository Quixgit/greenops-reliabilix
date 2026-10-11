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

export const intensityOrigin = z.object({ source: z.string(), basis: z.string(), carbon_kg_co2e: z.number(), share_pct: z.number() });
export const carbonSummary = totals.extend({
  has_data: z.boolean(), previous: totals, comparable: z.boolean(), methodology: methodologyInfo,
  intensity_origins: z.array(intensityOrigin).default([]), intensity_measured_share_pct: z.number().nullable().default(null),
});
export type CarbonSummary = z.infer<typeof carbonSummary>;

// Grid carbon intensity history of one region, one trusted source per day, other sources beside it (never blended).
export const intensityPoint = z.object({
  day: z.string(), g_per_kwh: z.number(), min: z.number(), max: z.number(), readings: z.number(),
  source: z.string(), basis: z.enum(["day_mean", "partial_day_mean", "month_mean"]), others: z.array(z.object({ source: z.string(), g_per_kwh: z.number() })),
});
export type IntensityPoint = z.infer<typeof intensityPoint>;
export const intensityHistory = z.object({
  region: z.string(), regions: z.array(z.object({ region: z.string(), carbon_kg_co2e: z.number() })), points: z.array(intensityPoint),
  policy: z.object({ priority: z.array(z.string()), min_day_readings: z.number(), max_age_hours: z.number() }),
  window_days: z.number(), days_with_data: z.number(),
});
export type IntensityHistory = z.infer<typeof intensityHistory>;

export const scheduleAdvice = z.object({
  region: z.string(), source: z.string(), basis: z.string(),
  window: z.object({
    start: z.string(), end: z.string(), average_g_per_kwh: z.number(), now_start: z.string(), now_average_g_per_kwh: z.number(),
    saving_pct: z.number(), points: z.number(),
  }),
});
export type ScheduleAdvice = z.infer<typeof scheduleAdvice>;

export const finopsSummary = z.object({
  has_data: z.boolean(),
  total_cost: z.number(),
  previous_total_cost: z.number(),
  previous_comparable: z.boolean(),
  currency: z.string(),
  by_service: z.array(z.object({ service: z.string(), cost: z.number() })),
  excluded: z.array(z.object({ currency: z.string(), rows: z.number() })).default([]),
});
export type FinopsSummary = z.infer<typeof finopsSummary>;

/** A budget with the consumption of its current period (in the budget's own currency). */
export const budgetStatus = z.object({
  id: z.string(), project_id: z.string().nullable(), amount: z.number(), currency: z.string(),
  period: z.enum(["monthly", "quarterly", "yearly"]), created_at: z.string(),
  period_start: z.string(), period_end: z.string(), spent: z.number(), used_pct: z.number(), projected: z.number(),
  state: z.enum(["ok", "at_risk", "exceeded"]),
});
export type BudgetStatus = z.infer<typeof budgetStatus>;
export const budgetList = z.object({ items: z.array(budgetStatus) }).transform((o) => o.items);
export const budget = z.object({ id: z.string(), project_id: z.string().nullable(), amount: z.number(), currency: z.string(), period: z.string() });

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
// What one cloud connection delivered over the period: billed cost in the organization's currency and data coverage.
export const gridSources = items(z.object({ provider: z.string(), last_at: z.string(), regions: z.number() }));
export const connectionCosts = items(z.object({ connection_id: z.string(), cost: z.number(), days: z.number(), last_day: z.string() }));
export type ConnectionCost = z.infer<typeof connectionCosts>[number];
export type RegionIntensity = z.infer<typeof regions>[number];

export const regionFootprint = items(z.object({
  provider: z.string(), region: z.string(), cost: z.number(), energy_kwh: z.number(), carbon_kg_co2e: z.number(), g_per_kwh: z.number().nullable(),
  alternative: z.object({
    region: z.string(), g_per_kwh: z.number(), carbon_saving_kg: z.number(), carbon_saving_pct: z.number(),
    cost_estimated: z.boolean(), cost_delta: z.number().nullable().default(null), cost_basis: z.string().optional(), residency_ok: z.boolean().default(false), checks_required: z.array(z.string()),
  }).nullable(),
  relocation_blocked: z.string().optional(),
}));
export type RegionFootprint = z.infer<typeof regionFootprint>[number];

export const optimization = z.object({
  open_count: z.number(), approved_count: z.number(), potential_carbon_kg_month: z.number(), potential_saving_month: z.number(),
  cost_not_estimated_count: z.number(), auto_apply_count: z.number().default(0), verified_count: z.number(), not_realized_count: z.number(),
  verified_carbon_kg_month: z.number(), verified_saving_month: z.number(),
});
export type Optimization = z.infer<typeof optimization>;

export const efficiency = z.object({
  has_units: z.boolean(), unit: z.string(), units: z.number(), days: z.number(), mixed_units: z.boolean(),
  carbon_g_per_unit: z.number().nullable(), cost_per_unit: z.number().nullable(),
});
export type Efficiency = z.infer<typeof efficiency>;

export const activity = items(z.object({ id: z.number(), action: z.string(), target: z.string(), at: z.string() }));
export type Activity = z.infer<typeof activity>[number];

export const project = z.object({
  id: z.string(), name: z.string(), functional_unit: z.string(), team_id: z.string().nullish(), created_at: z.string(),
});
export const projects = items(project);
export type Project = z.infer<typeof project>;

// Organization structure: teams own projects, environments hold a project's connections, allocation rules split shared cost.
export const team = z.object({ id: z.string(), name: z.string(), created_at: z.string() });
export const teams = items(team);
export type Team = z.infer<typeof team>;
export const environment = z.object({
  id: z.string(), project_id: z.string(), name: z.string(),
  kind: z.enum(["production", "staging", "development", "test", "other"]), connection_ids: z.array(z.string()),
});
export const environments = items(environment);
export type Environment = z.infer<typeof environment>;
export const allocationRule = z.object({
  id: z.string(), name: z.string(), source_project_id: z.string(), service_name: z.string(),
  targets: z.array(z.object({ project_id: z.string(), weight: z.number() })),
});
export const allocationRules = items(allocationRule);
export type AllocationRule = z.infer<typeof allocationRule>;
export const allocationReport = z.object({
  currency: z.string(), from: z.string(), to: z.string(), excluded: z.number(),
  projects: z.array(z.object({ project_id: z.string(), direct: z.number(), allocated_in: z.number(), allocated_out: z.number(), total: z.number() })),
  environments: z.array(z.object({ project_id: z.string(), environment_id: z.string().nullable(), cost: z.number() })),
  teams: z.array(z.object({ team_id: z.string().nullable(), total: z.number() })),
});
export type AllocationReport = z.infer<typeof allocationReport>;

export const connection = z.object({
  id: z.string(), project_id: z.string(), provider: z.string(), account_ref: z.string(), credential_ref: z.string().optional(),
  // How the platform authenticates: role | access_key | dataset_share | service_account_key | export | service_principal.
  access: z.string().optional(),
  sync_status: z.string(), last_sync_at: z.string().nullable(), last_error: z.string().nullable().optional(),
});
export const connections = items(connection);

export const setup = z.object({
  external_id: z.string(),
  platform_account_id: z.string().optional(),
  trust_policy: z.record(z.string(), z.unknown()).optional(),
  permissions_policy: z.record(z.string(), z.unknown()),
  required_permissions: z.array(z.string()),
  steps: z.array(z.string()).optional(),
});
export const connectResponse = z.object({ connection: connection, setup });
export type ConnectResponse = z.infer<typeof connectResponse>;
export const auditEntry = z.object({ id: z.number(), actor_user_id: z.string(), action: z.string(), target: z.string(), created_at: z.string() });
export const auditLog = items(auditEntry);
export const tenantInfo = z.object({ id: z.string(), name: z.string(), plan: z.string(), created_at: z.string() });
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
  details: z.record(z.string(), z.string()).nullish(),
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

export const member = z.object({
  id: z.string(), user_id: z.string(), email: z.string(), role: z.string(), created_at: z.string(),
  custom_role: z.object({ id: z.string(), name: z.string() }).nullable().optional(),
});
export type Member = z.infer<typeof member>;
export const members = items(member);
// Roles: the permission catalog, what each built-in role grants and the organization's custom roles.
export const customRole = z.object({
  id: z.string(), name: z.string(), description: z.string(), permissions: z.array(z.string()), members: z.number(),
  created_by: z.string(), created_at: z.string(), updated_at: z.string(),
});
export type CustomRole = z.infer<typeof customRole>;
export const rolesView = z.object({
  permissions: z.array(z.object({ key: z.string(), group: z.string(), assignable: z.boolean() })),
  builtin: z.array(z.object({ role: z.string(), permissions: z.array(z.string()) })),
  custom: z.array(customRole),
});
export type RolesView = z.infer<typeof rolesView>;

// The organization's plan: subscription state, limits with usage, modules and the payment providers that are set up.
export const planView = z.object({
  plan: z.string(), name: z.string(), status: z.enum(["none", "active", "trialing", "past_due", "canceled"]), source: z.string(),
  trial_ends_at: z.string().nullable(), period_end: z.string().nullable(),
  limits: z.array(z.object({ resource: z.string(), key: z.string(), limit: z.number(), used: z.number() })),
  modules: z.array(z.object({ key: z.string(), enabled: z.boolean() })),
  providers: z.array(z.object({ id: z.string(), name: z.string(), plans: z.array(z.string()) })),
});
export type PlanView = z.infer<typeof planView>;
export const checkoutResult = z.object({ url: z.string().optional(), message: z.string().optional() });

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

// Operators' own access log and the data source registry (ADR-0032). Counts are null for sources without connections.
export const operatorAuditEntry = z.object({
  id: z.number(), at: z.string(), operator_sub: z.string(), operator_email: z.string(), role: z.string(), action: z.string(),
});
export type OperatorAuditEntry = z.infer<typeof operatorAuditEntry>;
export const operatorAudit = items(operatorAuditEntry);
export const dataSourceState = z.enum(["ready", "attention", "not_configured", "planned"]);
export type DataSourceState = z.infer<typeof dataSourceState>;
export const dataSourceAccess = z.enum(["free", "paid", "customer_provided", "internal"]);
export type DataSourceAccess = z.infer<typeof dataSourceAccess>;
export const dataSource = z.object({
  id: z.string(), name: z.string(), category: z.string(), access: dataSourceAccess, state: dataSourceState,
  metrics: z.array(z.string()), coverage: z.string(), terms: z.string(), note: z.string(), integration_id: z.string(),
  connections: z.number().nullable(), organizations: z.number().nullable(), in_error: z.number().nullable(), last_sync_at: z.string().nullable(),
});
export type DataSource = z.infer<typeof dataSource>;
export const dataSources = items(dataSource);

export const operatorRecord = z.object({
  subject: z.string(), email: z.string(), role: z.enum(["viewer", "admin"]), created_at: z.string(), last_seen_at: z.string().nullable(), actions_30d: z.number(),
});
export type OperatorRecord = z.infer<typeof operatorRecord>;
export const operatorRecords = items(operatorRecord);
export const operatorJobs = z.object({
  queues: z.array(z.object({
    name: z.string(), size: z.number(), pending: z.number(), active: z.number(), scheduled: z.number(), retry: z.number(), archived: z.number(),
    completed: z.number(), processed_today: z.number(), failed_today: z.number(), latency_seconds: z.number(), paused: z.boolean(),
  })),
  failed: z.array(z.object({
    id: z.string(), queue: z.string(), type: z.string(), state: z.string(), last_error: z.string(), retried: z.number(), max_retry: z.number(),
    last_failed_at: z.string().nullable(), next_attempt_at: z.string().nullable(),
  })),
  schedules: z.array(z.object({ spec: z.string(), type: z.string(), next: z.string(), prev: z.string().nullable() })),
});
export type OperatorJobs = z.infer<typeof operatorJobs>;

export const operatorPlan = z.object({
  id: z.string(), name: z.string(), description: z.string(), limits: z.record(z.string(), z.number()), modules: z.record(z.string(), z.boolean()),
  organizations: z.number(),
});
export type OperatorPlan = z.infer<typeof operatorPlan>;
export const operatorPlans = items(operatorPlan);
export const operatorSubscription = z.object({
  plan: z.string(), status: z.string(), source: z.string(), trial_ends_at: z.string().nullable(), period_end: z.string().nullable(),
  updated_by: z.string(), updated_at: z.string().nullable(),
});
export type OperatorSubscription = z.infer<typeof operatorSubscription>;
export const billingEvent = z.object({
  at: z.string(), provider: z.string(), kind: z.enum(["applied", "requested"]), organization_id: z.string().nullable(),
  organization_name: z.string(), plan: z.string(), detail: z.string(),
});
export const billingEvents = items(billingEvent);
export const methodologySet = z.object({
  version: z.string(), status: z.string(), provenance: z.string(), notes: z.string(), caveats: z.array(z.string()), pue: z.number(),
  energy_kwh_per_unit: z.record(z.string(), z.number()), cost_based_kwh_per_usd: z.record(z.string(), z.number()),
  embodied_g_per_kwh: z.number(), checksum: z.string(), active: z.boolean(),
});
export const methodologySets = items(methodologySet);
export const regionSources = z.object({
  items: z.array(z.object({
    region: z.string(), override: z.string(), effective: z.string(),
    sources: z.array(z.object({ provider: z.string(), hourly_readings_30d: z.number(), monthly_figures: z.number(), newest: z.string().nullable() })),
  })),
});
export const orgUsage = z.object({
  days: z.number(), unit_costs: z.record(z.string(), z.number()),
  items: z.array(z.object({
    tenant_id: z.string(), name: z.string(), plan: z.string(), usage_rows: z.number(), calculation_rows: z.number(), sync_runs: z.number(),
    deliveries: z.number(), reports: z.number(), k8s_samples: z.number(), members: z.number(), connections: z.number(), estimated_cost_usd: z.number(),
  })),
});
export const apiUsage = z.object({
  items: z.array(z.object({ provider: z.string(), day: z.string(), requests: z.number(), errors: z.number() })),
  daily_caps: z.record(z.string(), z.number()),
});

export const errorStatus = z.enum(["open", "resolved", "ignored"]);
export type ErrorStatus = z.infer<typeof errorStatus>;
export const operatorError = z.object({
  id: z.string(), service: z.string(), component: z.string(), severity: z.enum(["warning", "error", "critical"]), message: z.string(),
  status: errorStatus, occurrences: z.number(), first_seen: z.string(), last_seen: z.string(), tenant_id: z.string().nullable(),
  tenant_name: z.string().nullable(), last_request_id: z.string().nullable(), handled_by: z.string().nullable(), handled_at: z.string().nullable(),
});
export type OperatorError = z.infer<typeof operatorError>;
export const operatorErrors = items(operatorError);
// Kubernetes workload energy pushed by the agent (ADR-0030). Carbon is null when some hours had no grid data.
export const k8sWorkload = z.object({
  cluster: z.string(), namespace: z.string(), workload: z.string(), kind: z.string(),
  energy_kwh: z.number(), carbon_kg_co2e: z.number().nullable(), hours: z.number(), hours_without_grid_data: z.number(),
  cpu_utilization_pct: z.number().nullable().optional(), memory_utilization_pct: z.number().nullable().optional(), usage_hours: z.number().optional(),
});
export type K8sWorkload = z.infer<typeof k8sWorkload>;
export const k8sWorkloads = items(k8sWorkload);
export const k8sCluster = z.object({
  cluster: z.string(), workloads: z.number(), namespaces: z.number(), energy_kwh: z.number(),
  carbon_kg_co2e: z.number().nullable(), last_hour: z.string(),
});
export const k8sClusters = items(k8sCluster);

// External services managed in the operator console. A secret field never carries a value, only "configured".
export const integrationField = z.object({
  key: z.string(), label: z.string(), help: z.string().optional(), placeholder: z.string().optional(),
  secret: z.boolean(), required: z.boolean(), env: z.string(), default: z.string().optional(),
  configured: z.boolean(), source: z.enum(["console", "environment", "not_set"]),
  value: z.string().optional(), hint: z.string().optional(), updated_at: z.string().optional(), updated_by: z.string().optional(),
});
export const managedIntegration = z.object({
  id: z.string(), name: z.string(), description: z.string(), testable: z.boolean(), startup: z.boolean().default(false), prepared: z.boolean().default(false), ready: z.boolean(), fields: z.array(integrationField),
});
export type ManagedIntegration = z.infer<typeof managedIntegration>;
export type IntegrationField = z.infer<typeof integrationField>;
export const managedIntegrations = items(managedIntegration);
export const testResult = z.object({ ok: z.boolean(), message: z.string() });
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

// Result of reconciling one sync with the cloud's own totals (ADR-0024). "unchecked" is never shown as "ok".
export const dataQuality = z.object({
  status: z.enum(["ok", "mismatch", "unchecked"]),
  checked_days: z.number(),
  mismatch_days: z.number(),
  max_diff_pct: z.number(),
  mismatched_days: z.array(z.string()).nullish(),
  note: z.string().nullish(),
  checked_at: z.string(),
});
export type DataQuality = z.infer<typeof dataQuality>;
export const connectionDetail = z.object({
  connection: z.object({
    id: z.string(), project_id: z.string(), provider: z.string(), account_ref: z.string(), sync_status: z.string(),
    credential_ref: z.string().optional(), access: z.string().optional(),
    last_error: z.string().nullable(), last_sync_at: z.string().nullable(), synced_through: z.string().nullable(),
    data_quality: dataQuality.nullable(),
  }),
  // What the customer has to set up on their side (the steps, the permissions); there is no secret in it.
  setup: z.object({ steps: z.array(z.string()).nullish(), required_permissions: z.array(z.string()).nullish() }).optional(),
});
export const syncRun = z.object({
  id: z.string(), started_at: z.string(), finished_at: z.string().nullable(), status: z.string(), records: z.number(),
  error: z.string().nullable(), quality: dataQuality.nullable(),
});
export type SyncRun = z.infer<typeof syncRun>;
export const syncRuns = items(syncRun);

export const alert = z.object({
  id: z.string(),
  kind: z.enum(["budget", "carbon", "connection_error", "data_quality"]),
  severity: z.enum(["info", "warning", "critical"]),
  status: z.enum(["new", "acknowledged", "muted", "resolved"]),
  title: z.string(),
  detail: z.string(),
  first_seen_at: z.string(),
  last_seen_at: z.string(),
  resolved_at: z.string().nullable(),
});
export type Alert = z.infer<typeof alert>;
export const alerts = items(alert);

export const automationJobs = items(z.object({ id: z.string(), kind: z.string(), status: z.string() }));
export type AutomationJobLite = z.infer<typeof automationJobs>[number];

/** A planned change of an approved recommendation. The platform plans; a person applies it and reports the result. */
export const automationJob = z.object({
  id: z.string(), project_id: z.string(), recommendation_id: z.string(), kind: z.string(), status: z.string(),
  plan: z.object({ summary: z.string(), steps: z.array(z.string()).nullish(), terraform: z.string().nullish(), cli: z.array(z.string()).nullish() }),
  risk: z.object({ level: z.string(), factors: z.array(z.string()).nullish() }),
  rollback_plan: z.string().nullish(),
  created_by: z.string(), created_at: z.string(),
  approved_by: z.string().nullable().optional(), approved_at: z.string().nullable().optional(),
  finished_by: z.string().nullable().optional(), finished_at: z.string().nullable().optional(),
  result_note: z.string().nullable().optional(),
});
export type AutomationJob = z.infer<typeof automationJob>;
export const automationJobList = items(automationJob);
