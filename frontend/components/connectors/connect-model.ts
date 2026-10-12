// The rules of connecting a cloud account: what each provider and way of connecting asks for, and the request it makes.
import { z } from "zod";

const GUID = /^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$/;
const base = { project_id: z.string().min(1, "Choose a project") };

// Every provider has two ways. The first is the usual one and the same everywhere: the customer's own read-only
// credential (an IAM user key, a Google service account key, an Azure service principal). The secret is sent once,
// sealed by the platform and never shown again. The second is the older way: a role, a shared dataset, an export.
const awsKey = z.object({
  ...base, provider: z.literal("aws"),
  account_ref: z.string().trim().regex(/^\d{12}$/, "AWS account id: 12 digits"),
  access_key_id: z.string().trim().regex(/^AKIA[A-Z0-9]{16}$/, "Access key id: starts with AKIA, 20 characters (a long-term IAM user key)"),
  client_secret: z.string().trim().regex(/^[A-Za-z0-9/+=]{40}$/, "Secret access key: 40 characters"),
});
const awsRole = z.object({
  ...base, provider: z.literal("aws"),
  account_ref: z.string().trim().regex(/^\d{12}$/, "AWS account id: 12 digits"),
  credential_ref: z.string().trim().regex(/^arn:aws:iam::\d{12}:role\/Reliabilix[\w+=,.@-]{0,100}$/, "Enter the IAM role ARN; the role name must start with \"Reliabilix\" (not an access key)"),
});
// The export table the customer enabled in Cloud Billing: bq://<project>/<dataset>/<table>
const BQ_TABLE = /^bq:\/\/[a-z][a-z0-9-]{4,28}[a-z0-9]\/[A-Za-z0-9_]{1,1024}\/gcp_billing_export_(resource_)?v1_[A-Za-z0-9_]+$/;
const gcpKey = z.object({
  ...base, provider: z.literal("gcp"),
  account_ref: z.string().trim().regex(/^[a-z][a-z0-9-]{4,28}[a-z0-9]$/, "GCP project id: 6-30 lowercase letters, digits or hyphens"),
  credential_ref: z.string().trim().regex(BQ_TABLE, "Use bq://<project>/<dataset>/gcp_billing_export_v1_…"),
  client_secret: z.string().trim().refine((v) => {
    try { const k = JSON.parse(v); return k?.type === "service_account" && typeof k.private_key === "string" && typeof k.client_email === "string"; } catch { return false; }
  }, "Paste the whole JSON key file of a service account"),
});
const gcpShare = z.object({
  ...base, provider: z.literal("gcp"),
  account_ref: z.string().trim().regex(/^[a-z][a-z0-9-]{4,28}[a-z0-9]$/, "GCP project id: 6-30 lowercase letters, digits or hyphens"),
  credential_ref: z.string().trim().regex(BQ_TABLE, "Use bq://<project>/<dataset>/gcp_billing_export_v1_…"),
});
const azureExport = z.object({
  ...base, provider: z.literal("azure"),
  account_ref: z.string().trim().regex(GUID, "Azure subscription id: a GUID"),
  // The Cost Management export folder: azblob://<tenant id>@<storage account>/<container>/<export directory>
  credential_ref: z.string().trim().regex(/^azblob:\/\/[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}@[a-z0-9]{3,24}\/[a-z0-9-]{3,63}\/[\w.-]+(\/[\w.-]+)*$/, "Use azblob://<tenant id>@<storage account>/<container>/<export directory>"),
});
const azureSp = z.object({
  ...base, provider: z.literal("azure"),
  account_ref: z.string().trim().regex(GUID, "Azure subscription id: a GUID"),
  tenant_id: z.string().trim().regex(GUID, "Directory (tenant) id: a GUID"),
  client_id: z.string().trim().regex(GUID, "Application (client) id: a GUID"),
  client_secret: z.string().trim().regex(/^[\x21-\x7e]{8,256}$/, "The client secret value (not the secret id)"),
});
export type Provider = "aws" | "gcp" | "azure";
export type Mode = "own" | "other";
export type Values = { project_id: string; provider: Provider; account_ref: string; credential_ref: string; mode: Mode; tenant_id: string; client_id: string; client_secret: string; access_key_id: string };
export const EMPTY = { project_id: "", account_ref: "", credential_ref: "", mode: "own" as Mode, tenant_id: "", client_id: "", client_secret: "", access_key_id: "" };

export function schemaFor(v: Pick<Values, "provider" | "mode">) {
  if (v.provider === "gcp") return v.mode === "other" ? gcpShare : gcpKey;
  if (v.provider === "azure") return v.mode === "other" ? azureExport : azureSp;
  return v.mode === "other" ? awsRole : awsKey;
}

/** What the API takes: a reference plus, for the customer's own credential, a write-only secret. */
export function toRequest(v: Values) {
  const own = v.mode !== "other";
  if (v.provider === "azure" && own) {
    return { project_id: v.project_id, provider: "azure", account_ref: v.account_ref.trim(),
      credential_ref: `azcost://${v.tenant_id.trim().toLowerCase()}@${v.client_id.trim().toLowerCase()}`, client_secret: v.client_secret.trim() };
  }
  if (v.provider === "aws" && own) {
    return { project_id: v.project_id, provider: "aws", account_ref: v.account_ref.trim(),
      credential_ref: `awskey://${v.access_key_id.trim()}`, client_secret: v.client_secret.trim() };
  }
  if (v.provider === "gcp" && own) {
    return { project_id: v.project_id, provider: "gcp", account_ref: v.account_ref.trim(), credential_ref: v.credential_ref.trim(), client_secret: v.client_secret.trim() };
  }
  return { project_id: v.project_id, provider: v.provider, account_ref: v.account_ref, credential_ref: v.credential_ref };
}


/** The key of a plain-words hint for a failed check, picked from what the platform said (null: nothing to add). */
export function verifyHint(message: string): string | null {
  const m = message.toLowerCase();
  if (/denied|forbidden|unauthori|not authori|permission|access is not granted|403|401/.test(m)) return "cw.hint.denied";
  if (/not found|does not exist|no such|404/.test(m)) return "cw.hint.not_found";
  if (/no data|no rows|empty|not delivered|no export/.test(m)) return "cw.hint.no_data";
  if (/timeout|timed out|unreachable|could not reach|connection refused|temporar|unavailable|503|502/.test(m)) return "cw.hint.network";
  return null;
}
