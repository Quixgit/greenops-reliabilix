"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useState, type CSSProperties } from "react";
import { PolicyPanel } from "@/components/forms/policy-panel";
import { Drawer } from "@/components/connectors/drawer";
import { Card, CardHeader } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconBranch, IconCheck, IconCopy, IconKey, IconLeaf, IconPlus, IconShield, IconTrash, IconWarning, IconPlay } from "@/components/ui/icons";
import { Info } from "@/components/ui/info";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { useApiKeys } from "@/hooks/use-admin";
import { useGateHistory, useTryGate, type GateChange, type GateResult } from "@/hooks/use-cicd";
import { useIntensityHistory, useProjects } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { apiKeyCreated } from "@/lib/api/schemas";
import { nf, timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";

const VERDICT: Record<string, { chip: string; ring: string; text: string; label: string }> = {
  PASS: { chip: "bg-leaf-soft text-leaf-deep", ring: "border-leaf bg-leaf-soft/60", text: "text-leaf-deep", label: "cd.v.pass" },
  WARN: { chip: "bg-[#fdf3dc] text-[#a9741a]", ring: "border-pastel-amber bg-[#fdf3dc]/70", text: "text-[#a9741a]", label: "cd.v.warn" },
  BLOCK: { chip: "bg-[#fdecef] text-[#c25667]", ring: "border-pastel-rose bg-[#fdecef]/70", text: "text-[#c25667]", label: "cd.v.block" },
};
const KINDS = ["vcpu_hours", "memory_gb_hours", "storage_gb_months"] as const;

function Stat({ index, icon: Icon, tone, label, info, value, hint }: { index: number; icon: typeof IconLeaf; tone: string; label: string; info: string; value: React.ReactNode; hint?: string }) {
  return (
    <Card index={index} className="flex items-center gap-4 p-4">
      <Icon className={cn("size-11 shrink-0", tone)} strokeWidth={1.4} />
      <div className="min-w-0">
        <p className="flex items-center gap-1 text-[13px] font-medium text-slate-600">{label}<Info>{info}</Info></p>
        <p className="mt-0.5 text-2xl font-bold tracking-tight tabular-nums">{value}</p>
        {hint && <p className="truncate text-xs text-muted">{hint}</p>}
      </div>
    </Card>
  );
}

export function GateStats() {
  const t = useT();
  const q = useGateHistory("");
  const s = q.data?.summary;
  const rate = s && s.total > 0 ? (s.pass / s.total) * 100 : null;
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Stat index={0} icon={IconBranch} tone="text-[#5b8fdc]" label={t("cd.stat.checks")} info={t("cd.info.checks")} hint={t("cd.stat.checks_hint")}
        value={q.isLoading ? "–" : <CountUp value={s?.total ?? 0} format={(n) => String(Math.round(n))} />} />
      <Stat index={1} icon={IconCheck} tone="text-leaf-deep" label={t("cd.stat.pass_rate")} info={t("cd.info.pass_rate")} hint={s ? t("cd.stat.pass_hint", { count: s.pass }) : undefined}
        value={q.isLoading ? "–" : rate === null ? "—" : <CountUp value={rate} format={(n) => `${Math.round(n)}%`} />} />
      <Stat index={2} icon={IconWarning} tone="text-[#d9a441]" label={t("cd.stat.warned")} info={t("cd.info.warned")} hint={t("cd.stat.warned_hint")}
        value={q.isLoading ? "–" : <CountUp value={s?.warn ?? 0} format={(n) => String(Math.round(n))} />} />
      <Stat index={3} icon={IconShield} tone="text-[#d9707f]" label={t("cd.stat.blocked")} info={t("cd.info.blocked")}
        hint={s && s.block > 0 ? t("cd.stat.kept_out", { kg: nf(1).format(s.carbon_blocked_kg_month) }) : t("cd.stat.nothing_blocked")}
        value={q.isLoading ? "–" : <CountUp value={s?.block ?? 0} format={(n) => String(Math.round(n))} />} />
    </div>
  );
}

const field = "w-full rounded-xl border border-line bg-white px-3 py-2 text-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-300 focus:border-leaf focus:shadow-[0_0_0_3px_rgba(111,195,168,0.22)]";

/** Ask the gate the way a pipeline does: describe a planned change, read PASS / WARN / BLOCK and why. */
export function GatePlayground() {
  const t = useT();
  const projects = useProjects().data ?? [];
  const regions = (useIntensityHistory(undefined).data?.regions ?? []).map((r) => r.region);
  const run = useTryGate();
  const [project, setProject] = useState("");
  const [rows, setRows] = useState<{ kind: GateChange["kind"]; amount: string; region: string; cost: string }[]>([{ kind: "vcpu_hours", amount: "7200", region: "", cost: "38" }]);
  const [override, setOverride] = useState({ warn: "", block: "" });
  const [res, setRes] = useState<GateResult | null>(null);
  useEffect(() => { if (projects.length && !project) setProject(projects[0].id); }, [projects, project]);
  useEffect(() => { if (regions.length) setRows((r) => r.map((x) => (x.region ? x : { ...x, region: regions[0] }))); }, [regions.length]); // eslint-disable-line react-hooks/exhaustive-deps

  const valid = !!project && rows.length > 0 && rows.every((r) => Number(r.amount) >= 0 && r.amount !== "" && r.region.trim() !== "");
  const submit = () => {
    const changes: GateChange[] = rows.map((r) => ({ kind: r.kind, amount: Number(r.amount), region: r.region.trim(), monthly_cost_delta: Number(r.cost) || 0 }));
    const thresholds: Record<string, number> = {};
    if (override.warn !== "") thresholds.carbon_warn_kg = Number(override.warn);
    if (override.block !== "") thresholds.carbon_block_kg = Number(override.block);
    run.mutate({ project_id: project, changes, thresholds: Object.keys(thresholds).length ? thresholds : undefined }, { onSuccess: setRes });
  };
  const v = res ? VERDICT[res.verdict] : null;
  return (
    <Card index={4} interactive={false}>
      <CardHeader icon={<IconPlay className="size-[18px] text-leaf-deep" />} title={t("cd.try.title")} info={t("cd.info.try")} />
      <p className="mb-4 text-sm leading-relaxed text-muted">{t("cd.try.hint")}</p>
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <div className="space-y-4">
          <label className="block text-xs font-medium text-slate-500">{t("cd.f.project")}
            <select value={project} onChange={(e) => setProject(e.target.value)} className={cn(field, "mt-1")}>{projects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}</select>
          </label>
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{t("cd.f.changes")}</p>
            {rows.map((r, i) => (
              <div key={i} className="grid grid-cols-[1.2fr_1fr_1.2fr_0.9fr_auto] items-end gap-2">
                <select value={r.kind} aria-label={t("cd.f.kind")} onChange={(e) => setRows(rows.map((x, j) => (j === i ? { ...x, kind: e.target.value as GateChange["kind"] } : x)))} className={field}>
                  {KINDS.map((k) => <option key={k} value={k}>{t(`cd.kind.${k}`)}</option>)}
                </select>
                <input value={r.amount} inputMode="decimal" aria-label={t("cd.f.amount")} placeholder={t("cd.f.amount")} onChange={(e) => setRows(rows.map((x, j) => (j === i ? { ...x, amount: e.target.value } : x)))} className={field} />
                <input value={r.region} list="cd-regions" aria-label={t("cd.f.region")} placeholder="eu-central-1" onChange={(e) => setRows(rows.map((x, j) => (j === i ? { ...x, region: e.target.value } : x)))} className={field} />
                <input value={r.cost} inputMode="decimal" aria-label={t("cd.f.cost")} placeholder={t("cd.f.cost")} onChange={(e) => setRows(rows.map((x, j) => (j === i ? { ...x, cost: e.target.value } : x)))} className={field} />
                <button type="button" aria-label={t("cd.f.remove")} disabled={rows.length === 1} onClick={() => setRows(rows.filter((_, j) => j !== i))} className="grid size-9 place-items-center rounded-full text-slate-400 transition-colors hover:bg-[#fdecef] hover:text-[#d9707f] disabled:invisible"><IconTrash className="size-4" strokeWidth={1.8} /></button>
              </div>
            ))}
            <datalist id="cd-regions">{regions.map((r) => <option key={r} value={r} />)}</datalist>
            <button type="button" onClick={() => setRows([...rows, { kind: "vcpu_hours", amount: "", region: regions[0] ?? "", cost: "" }])} className="inline-flex items-center gap-1 text-sm font-medium text-leaf-deep hover:underline"><IconPlus className="size-4" />{t("cd.f.add")}</button>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <label className="block text-xs font-medium text-slate-500">{t("cd.f.warn")}<input value={override.warn} inputMode="decimal" onChange={(e) => setOverride({ ...override, warn: e.target.value })} placeholder={t("cd.f.policy_default")} className={cn(field, "mt-1")} /></label>
            <label className="block text-xs font-medium text-slate-500">{t("cd.f.block")}<input value={override.block} inputMode="decimal" onChange={(e) => setOverride({ ...override, block: e.target.value })} placeholder={t("cd.f.policy_default")} className={cn(field, "mt-1")} /></label>
          </div>
          <div className="flex items-center gap-3">
            <button type="button" disabled={!valid || run.isPending} onClick={submit} className="btn-leaf rounded-xl px-5 py-2.5 text-sm font-semibold">{run.isPending ? t("cd.try.running") : t("cd.try.run")}</button>
            {run.isError && <span role="alert" className="text-sm text-bad">{run.error.message}</span>}
          </div>
        </div>

        <div className="min-h-[220px]">
          {!res || !v ? (
            <div className="flex h-full flex-col items-center justify-center rounded-2xl border border-dashed border-line px-6 py-10 text-center text-sm text-muted">
              <IconShield className="mb-3 size-12 text-slate-300" strokeWidth={1.3} />{t("cd.try.empty")}
            </div>
          ) : (
            <div key={`${res.verdict}${res.carbon_kg_month}`} className={cn("form-swap rounded-2xl border p-5", v.ring)}>
              <div className="flex items-center gap-3">
                <span className={cn("grid size-12 place-items-center rounded-full bg-white shadow-sm", v.text)}>{res.verdict === "PASS" ? <IconCheck className="pop size-7" strokeWidth={2.6} /> : <IconWarning className="pop size-7" strokeWidth={2} />}</span>
                <div><p className={cn("text-2xl font-bold tracking-tight", v.text)}>{res.verdict}</p><p className="text-xs text-slate-600">{t(`cd.verdict.${res.verdict}`)}</p></div>
              </div>
              <dl className="mt-4 grid grid-cols-2 gap-3 text-sm">
                <div className="rounded-xl bg-white/80 px-3 py-2"><dt className="text-xs text-muted">{t("cd.res.carbon")}</dt><dd className="font-semibold tabular-nums">{nf(1).format(res.carbon_kg_month)} kg CO₂e</dd></div>
                <div className="rounded-xl bg-white/80 px-3 py-2"><dt className="text-xs text-muted">{t("cd.res.cost")}</dt><dd className="font-semibold tabular-nums">{res.cost_delta_month >= 0 ? "+" : ""}{nf(0).format(res.cost_delta_month)}</dd></div>
              </dl>
              {!!res.reasons?.length && <ul className="mt-3 space-y-1 text-sm text-slate-700">{res.reasons.map((r, i) => <li key={i} className="flex gap-2"><span className="mt-2 size-1.5 shrink-0 rounded-full bg-slate-400" />{r}</li>)}</ul>}
              {!res.thresholds_applied && <p className="mt-3 text-xs text-muted">{t("cd.res.no_thresholds")}</p>}
              <p className="mt-3 text-[11px] leading-relaxed text-muted">{t("cd.res.method", { v: res.methodology_version })}{!res.embodied_carbon_included && ` ${t("cd.res.operational")}`}</p>
            </div>
          )}
        </div>
      </div>
    </Card>
  );
}

const policyView = z.object({
  ci_carbon_warn_kg: z.number().nullable().optional(), ci_carbon_block_kg: z.number().nullable().optional(),
  ci_cost_warn: z.number().nullable().optional(), ci_cost_block: z.number().nullable().optional(),
});

function PolicyRow({ id, name, canEdit, onEdit }: { id: string; name: string; canEdit: boolean; onEdit: () => void }) {
  const t = useT();
  const q = useQuery({ queryKey: ["policy", id], queryFn: () => api(`projects/${id}/policy`, policyView) });
  const p = q.data;
  const chip = (label: string, v: number | null | undefined, unit: string, tone: string) => v == null ? null : <span className={cn("rounded-full px-2.5 py-1 text-xs font-semibold tabular-nums", tone)}>{label} {nf(v < 10 ? 1 : 0).format(v)} {unit}</span>;
  const any = !!p && (p.ci_carbon_warn_kg != null || p.ci_carbon_block_kg != null || p.ci_cost_warn != null || p.ci_cost_block != null);
  return (
    <li className="flex flex-wrap items-center gap-3 py-3">
      <span className="min-w-0 flex-1 basis-40 truncate text-sm font-semibold">{name}</span>
      <span className="flex flex-wrap items-center gap-1.5">
        {q.isLoading ? <Skeleton className="h-6 w-40" /> : any ? (<>
          {chip(t("cd.pol.warn"), p!.ci_carbon_warn_kg, "kg", VERDICT.WARN.chip)}{chip(t("cd.pol.block"), p!.ci_carbon_block_kg, "kg", VERDICT.BLOCK.chip)}
          {chip(t("cd.pol.cost_warn"), p!.ci_cost_warn, "", VERDICT.WARN.chip)}{chip(t("cd.pol.cost_block"), p!.ci_cost_block, "", VERDICT.BLOCK.chip)}
        </>) : <span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-500">{t("cd.pol.none")}</span>}
      </span>
      <button type="button" onClick={onEdit} className="rounded-lg px-3 py-1.5 text-xs font-semibold text-leaf-deep ring-1 ring-line transition-colors hover:bg-leaf-soft">{canEdit ? t("cd.pol.edit") : t("cd.pol.view")}</button>
    </li>
  );
}

/** The limits the gate applies per project; a request may override them, the project policy is the default. */
export function GatePolicies({ canEdit }: { canEdit: boolean }) {
  const t = useT();
  const q = useProjects();
  const [open, setOpen] = useState<string | null>(null);
  const list = q.data ?? [];
  return (
    <Card index={5} interactive={false}>
      <CardHeader icon={<IconShield className="size-[18px] text-leaf-deep" />} title={t("cd.pol.title")} info={t("cd.info.policies")} />
      {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> : list.length === 0 ? <EmptyState icon={IconShield} text={t("cd.pol.empty")} action={{ href: "/projects", label: t("cd.pol.open_projects") }} /> : (
        <ul className="divide-y divide-line/70">{list.map((p) => <PolicyRow key={p.id} id={p.id} name={p.name} canEdit={canEdit} onEdit={() => setOpen(p.id)} />)}</ul>
      )}
      <Drawer open={!!open} onClose={() => setOpen(null)} title={t("cd.pol.drawer", { name: list.find((p) => p.id === open)?.name ?? "" })} closeLabel={t("common.cancel")}>
        {open && <PolicyPanel projectId={open} canEdit={canEdit} />}
      </Drawer>
    </Card>
  );
}

const yaml = (project: string, base: string) => `# .github/workflows/carbon-gate.yml
name: carbon-gate
on: pull_request
jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - name: Ask the Reliabilix gate
        env:
          RELIABILIX_API_KEY: \${{ secrets.RELIABILIX_API_KEY }}
        run: |
          verdict=$(curl -sS -X POST ${base}/api/v1/ci/evaluate \\
            -H "Authorization: Bearer $RELIABILIX_API_KEY" -H "Content-Type: application/json" \\
            -d '{"project_id": "${project}", "ref": "'"$GITHUB_REPOSITORY#$GITHUB_RUN_ID"'",
                 "changes": [{"kind": "vcpu_hours", "amount": 7200, "region": "eu-central-1", "monthly_cost_delta": 38}]}' | jq -r .verdict)
          echo "gate: $verdict"
          [ "$verdict" != "BLOCK" ]`;
const gitlab = (project: string, base: string) => `carbon-gate:
  stage: test
  image: curlimages/curl:latest
  script:
    - |
      curl -sS -X POST ${base}/api/v1/ci/evaluate \\
        -H "Authorization: Bearer $RELIABILIX_API_KEY" -H "Content-Type: application/json" \\
        -d "{\\"project_id\\": \\"${project}\\", \\"ref\\": \\"$CI_PIPELINE_URL\\", \\"changes\\": [{\\"kind\\": \\"vcpu_hours\\", \\"amount\\": 7200, \\"region\\": \\"eu-central-1\\", \\"monthly_cost_delta\\": 38}]}" | tee gate.json
    - grep -q '"verdict":"BLOCK"' gate.json && exit 1 || true`;
const schedule = (base: string) => `# When is the grid cleanest for a 2 hour job in the next 24 hours?
curl -sS "${base}/api/v1/ci/schedule?region=eu-central-1&duration_hours=2&within_hours=24" \\
  -H "Authorization: Bearer $RELIABILIX_API_KEY"`;

/** Everything a pipeline needs: a key that may only ask the gate, and ready snippets with the project filled in. */
export function Integrate({ canKeys }: { canKeys: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const keys = useApiKeys();
  const projects = useProjects().data ?? [];
  const [tab, setTab] = useState<"github" | "gitlab" | "schedule">("github");
  const [project, setProject] = useState("");
  const [fresh, setFresh] = useState("");
  const [copied, setCopied] = useState(false);
  const base = "https://<your Reliabilix API address>";
  useEffect(() => { if (projects.length && !project) setProject(projects[0].id); }, [projects, project]);
  const create = useMutation({
    mutationFn: () => api("api-keys", apiKeyCreated, undefined, { method: "POST", body: JSON.stringify({ name: `ci-${new Date().toISOString().slice(0, 10)}`, role: "ci" }) }),
    onSuccess: (r) => { setFresh(r.key); void qc.invalidateQueries({ queryKey: ["api-keys"] }); },
  });
  const ciKeys = (keys.data ?? []).filter((k) => k.role === "ci" && !k.revoked_at);
  const code = tab === "github" ? yaml(project || "<project id>", base) : tab === "gitlab" ? gitlab(project || "<project id>", base) : schedule(base);
  return (
    <Card index={6} interactive={false}>
      <CardHeader icon={<IconBranch className="size-[18px] text-[#5b8fdc]" />} title={t("cd.int.title")} info={t("cd.info.integrate")} />
      <div className="grid gap-6 lg:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)]">
        <div className="space-y-4">
          <div>
            <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("cd.int.keys")}</p>
            {keys.isLoading ? <Skeleton className="h-16" /> : ciKeys.length === 0 ? <p className="text-sm text-muted">{t("cd.int.no_keys")}</p> : (
              <ul className="space-y-2">{ciKeys.map((k) => (
                <li key={k.id} className="flex items-center gap-3 rounded-xl bg-slate-50 px-3 py-2 text-sm">
                  <IconKey className="size-5 shrink-0 text-leaf-deep" strokeWidth={1.5} />
                  <span className="min-w-0 flex-1"><span className="block truncate font-medium">{k.name}</span><span className="block text-xs text-muted"><code>{k.prefix}…</code> · {k.last_used_at ? t("cd.int.used", { ago: timeAgo(k.last_used_at) }) : t("cd.int.unused")}</span></span>
                </li>))}
              </ul>
            )}
          </div>
          {fresh && (
            <div className="rounded-xl bg-[#fdf3dc] p-3 text-sm">
              <p className="font-semibold text-[#a9741a]">{t("cd.int.fresh")}</p>
              <code className="mt-1 block break-all rounded-lg bg-white px-3 py-2 font-mono text-xs">{fresh}</code>
              <p className="mt-1 text-xs text-[#a9741a]">{t("cd.int.fresh_note")}</p>
            </div>
          )}
          {canKeys && <button type="button" disabled={create.isPending} onClick={() => create.mutate()} className="btn-leaf inline-flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold"><IconPlus className="size-4" strokeWidth={2.2} />{t("cd.int.create")}</button>}
          {create.isError && <p role="alert" className="text-sm text-bad">{create.error.message}</p>}
          <p className="text-xs leading-relaxed text-muted">{t("cd.int.scope")} <Link href="/settings" className="text-leaf-deep hover:underline">{t("cd.int.manage")}</Link></p>
        </div>
        <div>
          <div className="mb-3 flex flex-wrap items-center gap-3">
            <div role="tablist" className="flex gap-1 rounded-xl bg-slate-100 p-1 text-sm">
              {(["github", "gitlab", "schedule"] as const).map((k) => (
                <button key={k} role="tab" type="button" aria-selected={tab === k} onClick={() => setTab(k)} className={cn("rounded-lg px-3 py-1.5 font-medium transition-all", tab === k ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink")}>{t(`cd.int.tab.${k}`)}</button>
              ))}
            </div>
            {tab !== "schedule" && (
              <select value={project} onChange={(e) => setProject(e.target.value)} aria-label={t("cd.f.project")} className="ml-auto rounded-xl border border-line bg-white px-3 py-1.5 text-sm">
                {projects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
              </select>
            )}
          </div>
          <div className="relative">
            <button type="button" onClick={() => { void navigator.clipboard?.writeText(code); setCopied(true); window.setTimeout(() => setCopied(false), 1500); }}
              className="absolute right-2 top-2 inline-flex items-center gap-1 rounded-md bg-white/10 px-2 py-1 text-xs font-medium text-slate-200 transition-colors hover:bg-white/20">
              {copied ? <IconCheck className="size-3.5" strokeWidth={2.6} /> : <IconCopy className="size-3.5" />}{copied ? t("au.copied") : t("au.copy")}
            </button>
            <pre key={tab} className="form-swap max-h-[22rem] overflow-auto rounded-xl bg-slate-900 px-4 py-4 pr-20 font-mono text-xs leading-relaxed text-slate-100">{code}</pre>
          </div>
          <p className="mt-2 text-xs text-muted">{t("cd.int.snippet_note")}</p>
        </div>
      </div>
    </Card>
  );
}

/** What the gate decided and why, newest first. */
export function GateHistory() {
  const t = useT();
  const [verdict, setVerdict] = useState("");
  const q = useGateHistory(verdict);
  const projects = useProjects().data ?? [];
  const items = q.data?.items ?? [];
  const pg = usePaged(items, 10);
  const chip = (on: boolean) => cn("rounded-full px-3 py-1 text-xs font-semibold transition-all", on ? "bg-leaf-deep text-white" : "bg-slate-100 text-slate-600 hover:bg-leaf-soft hover:text-leaf-deep");
  return (
    <Card index={7} interactive={false}>
      <CardHeader icon={<IconCheck className="size-[18px] text-leaf-deep" />} title={t("cd.hist.title")} info={t("cd.info.history")} />
      <div className="mb-3 flex flex-wrap gap-1.5" role="group" aria-label={t("cd.hist.verdict")}>
        {["", "PASS", "WARN", "BLOCK"].map((v) => <button key={v} type="button" aria-pressed={verdict === v} className={chip(verdict === v)} onClick={() => setVerdict(v)}>{v ? t(VERDICT[v].label) : t("cd.hist.all")}</button>)}
      </div>
      {q.isLoading ? <Skeleton className="h-40" /> : q.isError ? <ErrorState /> : items.length === 0 ? (
        <EmptyState icon={IconBranch} text={verdict ? t("cd.hist.none_match") : t("cd.hist.empty")} />
      ) : (
        <ul key={`${verdict}${pg.page}`} className="divide-y divide-line/70">
          {pg.slice.map((e, i) => (
            <li key={e.id} className="row-in flex flex-wrap items-center gap-x-4 gap-y-1 py-3" style={{ "--i": i } as CSSProperties}>
              <span className={cn("w-16 shrink-0 rounded-full px-2.5 py-1 text-center text-[11px] font-bold", VERDICT[e.verdict].chip)}>{e.verdict}</span>
              <span className="min-w-0 flex-1 basis-56">
                <span className="block truncate text-sm font-semibold">{projects.find((p) => p.id === e.project_id)?.name ?? "—"}{e.ref && <span className="ml-2 font-normal text-muted">{e.ref}</span>}</span>
                <span className="block truncate text-xs text-muted">{e.reasons[0] ?? t("cd.hist.within")} · {e.regions.join(", ")}</span>
              </span>
              <span className="text-sm tabular-nums text-slate-700">{nf(1).format(e.carbon_kg_month)} kg · {e.cost_delta_month >= 0 ? "+" : ""}{nf(0).format(e.cost_delta_month)}</span>
              <span className="w-28 shrink-0 text-right text-xs text-muted">{e.source === "api_key" ? t("cd.hist.pipeline") : t("cd.hist.console")} · {timeAgo(e.created_at)}</span>
            </li>
          ))}
        </ul>
      )}
      <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
    </Card>
  );
}
