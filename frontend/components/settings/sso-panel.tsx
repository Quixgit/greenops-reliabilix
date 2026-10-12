"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { z } from "zod";
import { Panel } from "./organization-tab";
import { Badge } from "@/components/ui/badge";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { IconShield } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { api, apiVoid } from "@/lib/api/client";
import { useT } from "@/i18n/provider";

const domain = z.object({ id: z.string(), domain: z.string(), connection: z.string(), jit_role: z.string().default(""), verified: z.boolean(), record_name: z.string().optional(), record_value: z.string().optional() });
const list = z.object({ items: z.array(domain) });
const input = "h-10 w-full rounded-xl border border-line bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-leaf/40";

/** Company sign-in: the organization claims its e-mail domain (proved in DNS) and names the identity-provider connection its people sign in with. */
export function SsoPanel({ canEdit }: { canEdit: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["sso-domains"], queryFn: () => api("sso/domains", list) });
  const [dom, setDom] = useState("");
  const [conn, setConn] = useState("");
  const [jit, setJit] = useState("");
  const refresh = () => void qc.invalidateQueries({ queryKey: ["sso-domains"] });
  const add = useMutation({ mutationFn: () => api("sso/domains", domain, undefined, { method: "POST", body: JSON.stringify({ domain: dom.trim(), connection: conn.trim(), jit_role: jit }) }), onSuccess: () => { setDom(""); refresh(); } });
  const verify = useMutation({ mutationFn: (id: string) => apiVoid(`sso/domains/${id}/verify`, { method: "POST", body: "{}" }), onSuccess: refresh });
  const remove = useMutation({ mutationFn: (id: string) => apiVoid(`sso/domains/${id}`, { method: "DELETE" }), onSuccess: refresh });
  return (
    <Panel index={0} icon={IconShield} title={t("sso.title")} hint={t("sso.hint")} info={t("sso.info")}>
      {q.isLoading ? <Skeleton className="h-24" /> : (
        <div className="space-y-4">
          {(q.data?.items ?? []).map((d) => (
            <div key={d.id} className="space-y-2 rounded-2xl bg-slate-50/70 p-4 ring-1 ring-line/60">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-sm font-medium">{d.domain} <span className="font-normal text-muted">→ {d.connection}</span> {d.jit_role && <span className="text-xs text-muted">· {t("sso.jit", { role: d.jit_role })}</span>}</p>
                <span className="flex items-center gap-2">
                  <Badge tone={d.verified ? "good" : "warn"}>{d.verified ? t("sso.verified") : t("sso.pending")}</Badge>
                  {canEdit && <ConfirmButton label={t("sso.remove")} confirmLabel={t("sso.remove_confirm")} onConfirm={() => remove.mutate(d.id)} />}
                </span>
              </div>
              {!d.verified && d.record_name && (
                <div className="space-y-2 text-xs text-slate-600">
                  <p>{t("sso.dns")}</p>
                  <pre className="overflow-x-auto rounded-lg bg-white p-2.5 text-[11px] leading-snug ring-1 ring-line/60">{`TXT  ${d.record_name}\n${d.record_value}`}</pre>
                  {canEdit && <button type="button" disabled={verify.isPending} onClick={() => verify.mutate(d.id)} className="rounded-xl btn-leaf px-3.5 py-1.5 text-xs font-semibold text-white disabled:opacity-60">{verify.isPending ? t("sso.checking") : t("sso.verify")}</button>}
                  {verify.isError && verify.variables === d.id && <p role="alert" className="text-xs text-[#a13a4b]">{verify.error.message}</p>}
                </div>
              )}
            </div>
          ))}
          {(q.data?.items.length ?? 0) === 0 && <p className="text-sm text-muted">{t("sso.none")}</p>}
          {canEdit ? (
            <form className="grid gap-3 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_11rem_auto]" onSubmit={(e) => { e.preventDefault(); if (dom.trim() && conn.trim()) add.mutate(); }}>
              <label className="space-y-1 text-xs font-medium text-slate-600">{t("sso.domain")}<input value={dom} onChange={(e) => setDom(e.target.value)} className={input} placeholder="acme.com" required /></label>
              <label className="space-y-1 text-xs font-medium text-slate-600">{t("sso.connection")}<input value={conn} onChange={(e) => setConn(e.target.value)} className={input} placeholder="acme-saml" required maxLength={100} /></label>
              <label className="space-y-1 text-xs font-medium text-slate-600">{t("sso.join")}
                <select value={jit} onChange={(e) => setJit(e.target.value)} className={input}>
                  <option value="">{t("sso.join_none")}</option><option value="viewer">viewer</option><option value="engineer">engineer</option><option value="billing">billing</option>
                </select>
              </label>
              <button disabled={add.isPending} className="self-end h-10 rounded-xl btn-leaf px-4 text-sm font-semibold text-white disabled:opacity-60">{add.isPending ? t("sso.adding") : t("sso.add")}</button>
            </form>
          ) : <p className="text-sm text-muted">{t("sso.readonly")}</p>}
          {add.isError && <p role="alert" className="text-sm text-bad">{add.error.message}</p>}
          <p className="text-[11px] leading-snug text-muted">{t("sso.note")}</p>
        </div>
      )}
    </Panel>
  );
}
