"use client";

import { SelectField } from "@/components/ui/select-field";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { IconNetwork as Network, IconShare as Share2, IconDatabase as Layers } from "@/components/ui/icons";
import { useState } from "react";
import { z } from "zod";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useAllocation, useAllocationRules, useConnections, useEnvironments, useProjects, useTeams } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api, apiVoid } from "@/lib/api/client";
import { allocationRule, environment, team } from "@/lib/api/schemas";
import { formatCost } from "@/lib/formatters";
import { Field, input } from "./project-form";

/** The answer of a PUT is not used: only that it succeeded. */
const ack = z.object({}).passthrough();
const btn = "rounded-lg px-3 py-1.5 text-xs font-semibold ring-1 ring-line hover:bg-slate-50 disabled:opacity-60";
const KINDS = ["production", "staging", "development", "test", "other"] as const;

/** Invalidates everything that depends on the structure. */
function useRefresh() {
  const qc = useQueryClient();
  return () => { for (const k of ["projects", "teams", "environments", "allocation-rules", "allocation"]) void qc.invalidateQueries({ queryKey: [k] }); };
}

/** The cost of the period by project, with the allocation rules applied, and rolled up by team and environment. */
export function AllocationReportCard() {
  const t = useT();
  const q = useAllocation();
  const projects = useProjects();
  const teams = useTeams();
  const envs = useEnvironments();
  const pname = (id: string) => projects.data?.find((p) => p.id === id)?.name ?? id.slice(0, 8);
  const r = q.data;
  const cost = (v: number) => formatCost(v, r?.currency || "USD");
  const hasRules = r?.projects.some((p) => p.allocated_in !== 0 || p.allocated_out !== 0);
  return (
    <Card index={1}>
      <CardHeader info={t("info.struct_report")} title={t("structure.report.title")} />
      <p className="mb-3 text-sm text-muted">{t("structure.report.intro")}</p>
      {q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> : !r || r.projects.length === 0 ? (
        <EmptyState icon={Layers} text={t("structure.report.empty")} />
      ) : (
        <div className="space-y-5">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead><tr className="text-left text-xs uppercase tracking-wide text-slate-500">
                <th className="py-1.5 pr-3 font-semibold">{t("structure.report.project")}</th>
                <th className="py-1.5 pr-3 text-right font-semibold">{t("structure.report.direct")}</th>
                {hasRules && <><th className="py-1.5 pr-3 text-right font-semibold">{t("structure.report.in")}</th><th className="py-1.5 pr-3 text-right font-semibold">{t("structure.report.out")}</th></>}
                <th className="py-1.5 text-right font-semibold">{t("structure.report.total")}</th>
              </tr></thead>
              <tbody className="divide-y divide-line/70">
                {r.projects.map((p) => (
                  <tr key={p.project_id}>
                    <td className="py-2 pr-3 font-medium">{pname(p.project_id)}</td>
                    <td className="py-2 pr-3 text-right tabular-nums">{cost(p.direct)}</td>
                    {hasRules && <><td className="py-2 pr-3 text-right tabular-nums text-muted">{p.allocated_in ? `+${cost(p.allocated_in)}` : "–"}</td><td className="py-2 pr-3 text-right tabular-nums text-muted">{p.allocated_out ? `−${cost(p.allocated_out)}` : "–"}</td></>}
                    <td className="py-2 text-right font-semibold tabular-nums">{cost(p.total)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="grid gap-5 md:grid-cols-2">
            <div>
              <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("structure.report.by_team")}</p>
              <ul className="space-y-1 text-sm">
                {r.teams.map((x) => (
                  <li key={x.team_id ?? "none"} className="flex justify-between gap-3">
                    <span>{x.team_id ? teams.data?.find((y) => y.id === x.team_id)?.name ?? "?" : <span className="text-muted">{t("structure.no_team")}</span>}</span>
                    <span className="tabular-nums">{cost(x.total)}</span>
                  </li>
                ))}
              </ul>
            </div>
            <div>
              <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("structure.report.by_env")}</p>
              <ul className="space-y-1 text-sm">
                {r.environments.map((x) => {
                  const env = x.environment_id ? envs.data?.find((y) => y.id === x.environment_id) : undefined;
                  return (
                    <li key={`${x.project_id}|${x.environment_id}`} className="flex justify-between gap-3">
                      <span>{pname(x.project_id)} · {env ? <>{env.name} <Badge tone={env.kind === "production" ? "warn" : "neutral"}>{t(`structure.kind.${env.kind}`)}</Badge></> : <span className="text-muted">{t("structure.unassigned")}</span>}</span>
                      <span className="tabular-nums">{cost(x.cost)}</span>
                    </li>
                  );
                })}
              </ul>
            </div>
          </div>
          {r.excluded > 0 && <p className="text-xs text-bad">{t("structure.report.excluded", { count: r.excluded })}</p>}
          <p className="text-xs text-muted">{t("structure.report.note")}</p>
        </div>
      )}
    </Card>
  );
}

export function TeamsCard({ canEdit }: { canEdit: boolean }) {
  const t = useT();
  const q = useTeams();
  const refresh = useRefresh();
  const [name, setName] = useState("");
  const add = useMutation({
    mutationFn: () => api("teams", team, undefined, { method: "POST", body: JSON.stringify({ name: name.trim() }) }),
    onSuccess: () => { setName(""); refresh(); },
  });
  const del = useMutation({ mutationFn: (id: string) => apiVoid(`teams/${id}`, { method: "DELETE" }), onSuccess: refresh });
  return (
    <Card index={2}>
      <CardHeader info={t("info.struct_teams")} title={t("structure.teams.title")} />
      <p className="mb-3 text-sm text-muted">{t("structure.teams.intro")}</p>
      {canEdit && (
        <form onSubmit={(e) => { e.preventDefault(); if (name.trim()) add.mutate(); }} className="mb-3 flex flex-wrap items-end gap-3">
          <Field label={t("structure.teams.name")}><input value={name} onChange={(e) => setName(e.target.value)} maxLength={80} className={input} placeholder="Platform" /></Field>
          <button disabled={add.isPending || !name.trim()} className="brand-gradient rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{t("structure.teams.add")}</button>
        </form>
      )}
      {add.isError && <p role="alert" className="mb-2 text-sm text-bad">{add.error.message}</p>}
      {q.isLoading ? <Skeleton className="h-10" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ? <EmptyState icon={Network} text={t("structure.teams.empty")} /> : (
        <ul className="divide-y divide-line/70 text-sm">
          {q.data!.map((x) => (
            <li key={x.id} className="flex items-center justify-between gap-3 py-2">
              <span className="font-medium">{x.name}</span>
              {canEdit && <button className={btn} disabled={del.isPending} onClick={() => del.mutate(x.id)}>{t("structure.delete")}</button>}
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

/** Team and environments of one project, shown inside the project's expanded row. */
export function ProjectStructure({ projectId, teamId, canEdit }: { projectId: string; teamId: string | null | undefined; canEdit: boolean }) {
  const t = useT();
  const refresh = useRefresh();
  const teams = useTeams();
  const envs = useEnvironments();
  const conns = useConnections();
  const mine = (envs.data ?? []).filter((e) => e.project_id === projectId);
  const projectConns = (conns.data ?? []).filter((c) => c.project_id === projectId);
  const [name, setName] = useState("");
  const [kind, setKind] = useState<(typeof KINDS)[number]>("production");
  const setTeam = useMutation({
    mutationFn: (id: string) => api(`projects/${projectId}/team`, ack, undefined, { method: "PUT", body: JSON.stringify({ team_id: id || null }) }),
    onSuccess: refresh,
  });
  const create = useMutation({
    mutationFn: () => api("environments", environment, undefined, { method: "POST", body: JSON.stringify({ project_id: projectId, name: name.trim(), kind }) }),
    onSuccess: () => { setName(""); refresh(); },
  });
  const del = useMutation({ mutationFn: (id: string) => apiVoid(`environments/${id}`, { method: "DELETE" }), onSuccess: refresh });
  const assign = useMutation({
    mutationFn: ({ env, ids }: { env: string; ids: string[] }) => api(`environments/${env}/connections`, ack, undefined, { method: "PUT", body: JSON.stringify({ connection_ids: ids }) }),
    onSuccess: refresh,
  });
  return (
    <div className="space-y-3 rounded-xl bg-slate-50 p-3 ring-1 ring-line">
      <label className="flex flex-wrap items-center gap-2 text-sm">
        <span className="font-medium text-slate-700">{t("structure.project.team")}</span>
        <SelectField disabled={!canEdit || setTeam.isPending} value={teamId ?? ""} onChange={(e) => setTeam.mutate(e.target.value)} className="rounded-lg bg-white px-2.5 py-1.5 text-sm ring-1 ring-line">
          <option value="">{t("structure.no_team")}</option>
          {teams.data?.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
        </SelectField>
      </label>
      <div>
        <p className="mb-1 text-sm font-medium text-slate-700">{t("structure.env.title")}</p>
        {mine.length === 0 ? <p className="text-xs text-muted">{t("structure.env.empty")}</p> : (
          <ul className="space-y-2">
            {mine.map((e) => (
              <li key={e.id} className="rounded-lg bg-white p-2.5 text-sm ring-1 ring-line">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="font-medium">{e.name} <Badge tone={e.kind === "production" ? "warn" : "neutral"}>{t(`structure.kind.${e.kind}`)}</Badge></span>
                  {canEdit && <button className={btn} disabled={del.isPending} onClick={() => del.mutate(e.id)}>{t("structure.delete")}</button>}
                </div>
                {projectConns.length === 0 ? <p className="mt-1 text-xs text-muted">{t("structure.env.no_connections")}</p> : (
                  <ul className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs">
                    {projectConns.map((c) => {
                      const on = e.connection_ids.includes(c.id);
                      return (
                        <li key={c.id}><label className="flex items-center gap-1.5">
                          <input type="checkbox" checked={on} disabled={!canEdit || assign.isPending}
                            onChange={() => assign.mutate({ env: e.id, ids: on ? e.connection_ids.filter((x) => x !== c.id) : [...e.connection_ids, c.id] })} />
                          {c.provider.toUpperCase()} {c.account_ref}
                        </label></li>
                      );
                    })}
                  </ul>
                )}
              </li>
            ))}
          </ul>
        )}
        {canEdit && (
          <form onSubmit={(ev) => { ev.preventDefault(); if (name.trim()) create.mutate(); }} className="mt-2 flex flex-wrap items-center gap-2">
            <input value={name} onChange={(e) => setName(e.target.value)} maxLength={60} placeholder={t("structure.env.name")} aria-label={t("structure.env.name")} className="rounded-lg bg-white px-2.5 py-1.5 text-sm ring-1 ring-line" />
            <SelectField value={kind} onChange={(e) => setKind(e.target.value as typeof kind)} aria-label={t("structure.env.kind")} className="rounded-lg bg-white px-2.5 py-1.5 text-sm ring-1 ring-line">
              {KINDS.map((k) => <option key={k} value={k}>{t(`structure.kind.${k}`)}</option>)}
            </SelectField>
            <button disabled={create.isPending || !name.trim()} className={btn}>{t("structure.env.add")}</button>
          </form>
        )}
        {(create.isError || assign.isError || setTeam.isError || del.isError) && (
          <p role="alert" className="mt-1 text-xs text-bad">{(create.error ?? assign.error ?? setTeam.error ?? del.error)?.message}</p>
        )}
      </div>
    </div>
  );
}

export function AllocationRulesCard({ canEdit }: { canEdit: boolean }) {
  const t = useT();
  const q = useAllocationRules();
  const projects = useProjects();
  const refresh = useRefresh();
  const pname = (id: string) => projects.data?.find((p) => p.id === id)?.name ?? id.slice(0, 8);
  const [name, setName] = useState("");
  const [source, setSource] = useState("");
  const [service, setService] = useState("");
  const [weights, setWeights] = useState<Record<string, string>>({});
  const targets = Object.entries(weights).map(([project_id, w]) => ({ project_id, weight: Number(w.replace(",", ".")) })).filter((x) => Number.isFinite(x.weight) && x.weight > 0);
  const valid = name.trim() !== "" && source !== "" && targets.length > 0;
  const create = useMutation({
    mutationFn: () => api("allocation-rules", allocationRule, undefined, { method: "POST", body: JSON.stringify({ name: name.trim(), source_project_id: source, service_name: service.trim(), targets }) }),
    onSuccess: () => { setName(""); setService(""); setWeights({}); refresh(); },
  });
  const del = useMutation({ mutationFn: (id: string) => apiVoid(`allocation-rules/${id}`, { method: "DELETE" }), onSuccess: refresh });
  return (
    <Card index={3}>
      <CardHeader info={t("info.struct_rules")} title={t("structure.rules.title")} />
      <p className="mb-3 text-sm text-muted">{t("structure.rules.intro")}</p>
      {canEdit && (projects.data?.length ?? 0) > 1 && (
        <form onSubmit={(e) => { e.preventDefault(); if (valid) create.mutate(); }} className="mb-4 space-y-3 rounded-xl bg-slate-50 p-3 ring-1 ring-line">
          <div className="grid gap-3 sm:grid-cols-3">
            <Field label={t("structure.rules.name")}><input value={name} onChange={(e) => setName(e.target.value)} maxLength={120} className={input} placeholder="Shared support" /></Field>
            <Field label={t("structure.rules.source")}>
              <SelectField value={source} onChange={(e) => setSource(e.target.value)} className={input}>
                <option value="">{t("structure.rules.choose")}</option>
                {projects.data?.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
              </SelectField>
            </Field>
            <Field label={t("structure.rules.service")}><input value={service} onChange={(e) => setService(e.target.value)} maxLength={200} className={input} placeholder={t("structure.rules.service_all")} /></Field>
          </div>
          <div>
            <p className="mb-1 text-sm font-medium text-slate-600">{t("structure.rules.targets")}</p>
            <ul className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {projects.data?.map((p) => (
                <li key={p.id} className="flex items-center gap-2 text-sm">
                  <span className="min-w-0 flex-1 truncate">{p.name}</span>
                  <input value={weights[p.id] ?? ""} onChange={(e) => setWeights({ ...weights, [p.id]: e.target.value })} inputMode="decimal" placeholder="0" aria-label={`${t("structure.rules.weight")}: ${p.name}`} className="w-20 rounded-lg bg-white px-2 py-1.5 text-right text-sm ring-1 ring-line" />
                </li>
              ))}
            </ul>
            <p className="mt-1 text-xs text-muted">{t("structure.rules.weights_hint")}</p>
          </div>
          <button disabled={create.isPending || !valid} className="brand-gradient rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{t("structure.rules.add")}</button>
          {create.isError && <p role="alert" className="text-sm text-bad">{create.error.message}</p>}
        </form>
      )}
      {q.isLoading ? <Skeleton className="h-16" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ? <EmptyState icon={Share2} text={t("structure.rules.empty")} /> : (
        <ul className="divide-y divide-line/70 text-sm">
          {q.data!.map((r) => {
            const sum = r.targets.reduce((a, x) => a + x.weight, 0);
            return (
              <li key={r.id} className="space-y-1 py-3">
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <p className="font-medium">{r.name}</p>
                    <p className="text-muted">{pname(r.source_project_id)} · {r.service_name || t("structure.rules.service_all")}</p>
                  </div>
                  {canEdit && <button className={btn} disabled={del.isPending} onClick={() => del.mutate(r.id)}>{t("structure.delete")}</button>}
                </div>
                <p className="text-xs text-muted">{r.targets.map((x) => `${pname(x.project_id)} ${sum > 0 ? Math.round((x.weight / sum) * 100) : 0}%`).join(" · ")}</p>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
