"use client";

import { useState } from "react";
import { IconCheck, IconCopy } from "@/components/ui/icons";
import { useT } from "@/i18n/provider";
import { cn } from "@/lib/utils";
import type { Provider } from "./connect-model";

type Ctx = { account: string; table: string };
type Snippet = { id: "terraform" | "cli"; code: string };

const AWS_POLICY = `{
  "Version": "2012-10-17",
  "Statement": [{ "Effect": "Allow", "Action": ["ce:GetCostAndUsage", "ce:GetRightsizingRecommendation", "ce:GetSavingsPlansUtilization", "ce:GetSavingsPlansCoverage", "ce:GetReservationUtilization", "ce:GetReservationCoverage"], "Resource": "*" }]
}`;

/** The cloud-side access of the customer's own read-only credential, as code. Values the person typed are filled in; the rest are placeholders. */
export function snippets(provider: Provider, c: Ctx): Snippet[] {
  const acct = c.account.trim();
  if (provider === "aws") {
    return [
      { id: "terraform", code: `# Read-only IAM user for Reliabilix (cost data only, no resources). The key it creates goes into the form.
resource "aws_iam_user" "reliabilix" {
  name = "ReliabilixReadOnly"
}

resource "aws_iam_user_policy" "reliabilix" {
  name   = "ReliabilixCostRead"
  user   = aws_iam_user.reliabilix.name
  policy = <<-JSON
${AWS_POLICY}
  JSON
}

resource "aws_iam_access_key" "reliabilix" {
  user = aws_iam_user.reliabilix.name
}

output "access_key_id" { value = aws_iam_access_key.reliabilix.id }
output "secret_access_key" {
  value     = aws_iam_access_key.reliabilix.secret
  sensitive = true # terraform output -raw secret_access_key
}` },
      { id: "cli", code: `aws iam create-user --user-name ReliabilixReadOnly
aws iam put-user-policy --user-name ReliabilixReadOnly --policy-name ReliabilixCostRead --policy-document '${AWS_POLICY.replace(/\n\s*/g, " ")}'
aws iam create-access-key --user-name ReliabilixReadOnly   # AccessKeyId and SecretAccessKey go into the form
# Cost Explorer must be enabled once: Billing > Cost Explorer (the first data can take up to 24 hours).` },
    ];
  }
  if (provider === "azure") {
    const sub = acct || "<subscription id>";
    return [
      { id: "terraform", code: `# Service principal with the Cost Management Reader role on one subscription (cost data only).
data "azuread_client_config" "current" {}

resource "azuread_application" "reliabilix" {
  display_name = "reliabilix-readonly"
  owners       = [data.azuread_client_config.current.object_id]
}

resource "azuread_service_principal" "reliabilix" {
  client_id = azuread_application.reliabilix.client_id
  owners    = [data.azuread_client_config.current.object_id]
}

resource "azuread_application_password" "reliabilix" {
  application_id = azuread_application.reliabilix.id
  end_date       = timeadd(timestamp(), "8760h")
}

resource "azurerm_role_assignment" "reliabilix" {
  scope                = "/subscriptions/${sub}"
  role_definition_name = "Cost Management Reader"
  principal_id         = azuread_service_principal.reliabilix.object_id
}

output "tenant_id"     { value = data.azuread_client_config.current.tenant_id }
output "client_id"     { value = azuread_application.reliabilix.client_id }
output "client_secret" {
  value     = azuread_application_password.reliabilix.value
  sensitive = true
}` },
      { id: "cli", code: `az ad sp create-for-rbac --name reliabilix-readonly --role "Cost Management Reader" --scopes /subscriptions/${sub} --years 1
# The output has tenant, appId (client id) and password (client secret): they go into the form.` },
    ];
  }
  const m = /^bq:\/\/([^/]+)\/([^/]+)\//.exec(c.table.trim());
  const proj = acct || "<gcp project id>", billingProj = m?.[1] ?? "<billing project>", dataset = m?.[2] ?? "<dataset>";
  return [
    { id: "terraform", code: `# Service account that reads the billing export dataset (read-only) and runs the query in its own project.
resource "google_service_account" "reliabilix" {
  project      = "${proj}"
  account_id   = "reliabilix-readonly"
  display_name = "Reliabilix cost read"
}

resource "google_bigquery_dataset_iam_member" "reliabilix" {
  project    = "${billingProj}"
  dataset_id = "${dataset}"
  role       = "roles/bigquery.dataViewer"
  member     = "serviceAccount:\${google_service_account.reliabilix.email}"
}

resource "google_project_iam_member" "reliabilix_jobs" {
  project = "${proj}"
  role    = "roles/bigquery.jobUser"
  member  = "serviceAccount:\${google_service_account.reliabilix.email}"
}

resource "google_service_account_key" "reliabilix" {
  service_account_id = google_service_account.reliabilix.name
}

output "key_json" {
  value     = base64decode(google_service_account_key.reliabilix.private_key)
  sensitive = true # terraform output -raw key_json: paste the whole file into the form
}` },
    { id: "cli", code: `gcloud iam service-accounts create reliabilix-readonly --project ${proj}
SA=reliabilix-readonly@${proj}.iam.gserviceaccount.com
gcloud projects add-iam-policy-binding ${proj} --member serviceAccount:$SA --role roles/bigquery.jobUser
bq add-iam-policy-binding --member=serviceAccount:$SA --role=roles/bigquery.dataViewer ${billingProj}:${dataset}
gcloud iam service-accounts keys create key.json --iam-account $SA   # paste the whole key.json into the form` },
  ];
}

/** A collapsible "do it with code" block for the details step: Terraform or CLI, copyable. */
export function SetupAsCode({ provider, account, table }: { provider: Provider; account: string; table: string }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [tab, setTab] = useState<"terraform" | "cli">("terraform");
  const [done, setDone] = useState(false);
  const list = snippets(provider, { account, table });
  const cur = list.find((s) => s.id === tab) ?? list[0];
  return (
    <div className="mt-5 rounded-2xl border border-line bg-slate-50/60">
      <button type="button" aria-expanded={open} onClick={() => setOpen((v) => !v)} className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left text-sm font-semibold text-ink">
        <span>{t("sc.title")}<span className="ml-2 text-xs font-normal text-muted">{t("sc.sub")}</span></span>
        <span aria-hidden className={cn("text-slate-400 transition-transform duration-300", open && "rotate-180")}>⌄</span>
      </button>
      <div className={cn("grid transition-[grid-template-rows] duration-300 ease-out", open ? "grid-rows-[1fr]" : "grid-rows-[0fr]")}>
        <div className="overflow-hidden">
          <div className="px-4 pb-4">
            <div className="mb-2 flex items-center justify-between gap-2">
              <div className="flex gap-1 rounded-lg bg-slate-100 p-0.5" role="tablist">
                {list.map((s) => <button key={s.id} type="button" role="tab" aria-selected={tab === s.id} onClick={() => setTab(s.id)} className={cn("rounded-md px-3 py-1 text-xs font-semibold transition-all", tab === s.id ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink")}>{s.id === "terraform" ? "Terraform" : "CLI"}</button>)}
              </div>
              <button type="button" onClick={() => { void navigator.clipboard?.writeText(cur.code); setDone(true); window.setTimeout(() => setDone(false), 1500); }}
                className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-ink">
                {done ? <IconCheck className="size-3.5 text-leaf-deep" strokeWidth={2.6} /> : <IconCopy className="size-3.5" />}{done ? t("au.copied") : t("au.copy")}
              </button>
            </div>
            <pre className="max-h-72 overflow-auto whitespace-pre rounded-xl bg-slate-900 px-4 py-3 font-mono text-xs leading-relaxed text-slate-100">{cur.code}</pre>
            <p className="mt-2 text-xs text-muted">{t("sc.note")}</p>
          </div>
        </div>
      </div>
    </div>
  );
}
