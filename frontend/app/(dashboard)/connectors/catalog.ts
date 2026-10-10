// The connector catalog shown to customers. `available` means the code exists and is tested; everything else is
// labelled with its phase (docs/product/03-connectors.md) so the page never promises more than the platform does.
export type ConnectorCategory = "cloud" | "infrastructure" | "ci-cd" | "notifications" | "data";
export type ConnectorStatus = "available" | "platform" | "planned";

export interface CatalogItem {
  id: string;
  name: string;
  category: ConnectorCategory;
  provides: string;
  status: ConnectorStatus;
  /** Provider key used by /cloud-accounts, when the connector is a customer connection. */
  provider?: string;
  phase?: string;
}

export const CATALOG: CatalogItem[] = [
  { id: "aws", name: "Amazon Web Services", category: "cloud", provides: "Cost Explorer, FOCUS data export, rightsizing findings", status: "available", provider: "aws" },
  { id: "gcp", name: "Google Cloud", category: "cloud", provides: "Cloud Billing export in BigQuery, measured compute and storage", status: "available", provider: "gcp" },
  { id: "azure", name: "Microsoft Azure", category: "cloud", provides: "Cost Management API, subscriptions", status: "planned", phase: "Phase 2" },
  { id: "kubernetes", name: "Kubernetes", category: "infrastructure", provides: "Workloads, utilisation, Kepler energy", status: "planned", phase: "Phase 2" },
  { id: "prometheus", name: "Prometheus", category: "infrastructure", provides: "CPU and memory metrics", status: "planned", phase: "Phase 2" },
  { id: "github-actions", name: "GitHub Actions", category: "ci-cd", provides: "Workflow runs, builds, deployments", status: "planned", phase: "Phase 3" },
  { id: "gitlab-ci", name: "GitLab CI/CD", category: "ci-cd", provides: "Pipelines, jobs, deployments", status: "planned", phase: "Phase 3" },
  { id: "email", name: "E-mail (SMTP)", category: "notifications", provides: "Alerts and report notifications", status: "available" },
  { id: "slack", name: "Slack", category: "notifications", provides: "Incoming-webhook notifications", status: "available" },
  { id: "webhook", name: "Signed webhook", category: "notifications", provides: "JSON events with HMAC signature", status: "available" },
  { id: "teams", name: "Microsoft Teams", category: "notifications", provides: "Team notifications", status: "planned", phase: "Phase 2" },
  { id: "electricity-maps", name: "Electricity Maps", category: "data", provides: "Grid carbon intensity (managed by the platform)", status: "platform" },
  { id: "watttime", name: "WattTime", category: "data", provides: "Secondary grid-data provider", status: "planned", phase: "Phase 2" },
];

export const CATEGORIES: { id: ConnectorCategory | "all"; label: string }[] = [
  { id: "all", label: "All" },
  { id: "cloud", label: "Cloud providers" },
  { id: "infrastructure", label: "Infrastructure & monitoring" },
  { id: "ci-cd", label: "CI/CD & development" },
  { id: "notifications", label: "Notifications" },
  { id: "data", label: "Data sources" },
];
