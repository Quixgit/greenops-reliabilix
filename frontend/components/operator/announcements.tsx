"use client";

import { useState } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconBell, IconClock, IconMail, IconSend, IconWarning, IconBolt, type IconType } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorMe, useOperatorPlans } from "@/hooks/use-admin";
import { useAnnouncements, useSendAnnouncement, useWithdrawAnnouncement } from "@/hooks/use-operator-extra";
import { timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { BUILD_VERSION } from "@/lib/version";
import { PageTitle } from "./parts";
import { ConfirmButton } from "@/components/ui/confirm-button";

const KINDS: { id: "update" | "maintenance" | "notice" | "incident"; icon: IconType; tone: string; sev: "info" | "warning" | "critical"; title: string; body: string }[] = [
  { id: "update", icon: IconBolt, tone: "text-leaf-deep", sev: "info", title: `Reliabilix ${BUILD_VERSION.number} is out`, body: "What is new: " },
  { id: "maintenance", icon: IconClock, tone: "text-[#d9a441]", sev: "warning", title: "Planned maintenance", body: "The platform will be unavailable for changes on " },
  { id: "notice", icon: IconBell, tone: "text-[#5b8fdc]", sev: "info", title: "", body: "" },
  { id: "incident", icon: IconWarning, tone: "text-[#d9707f]", sev: "critical", title: "We are investigating an issue", body: "" },
];
const SEV_CHIP = { critical: "bg-[#fdecef] text-[#c25667]", warning: "bg-[#fdf3dc] text-[#a9741a]", info: "bg-[#eef3fd] text-[#4a6fb0]" } as const;
const field = "w-full rounded-xl border border-line bg-white px-3 py-2 text-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-300 focus:border-leaf focus:shadow-[0_0_0_3px_rgba(111,195,168,0.22)]";

/** Tell every organization something about the platform: a new release, maintenance, a notice or an incident. */
export function AnnouncementsView() {
  const me = useOperatorMe();
  const plans = useOperatorPlans();
  const list = useAnnouncements();
  const send = useSendAnnouncement();
  const withdraw = useWithdrawAnnouncement();
  const canAct = me.data?.role === "admin";
  const [kind, setKind] = useState<(typeof KINDS)[number]["id"]>("update");
  const [severity, setSeverity] = useState<"info" | "warning" | "critical">("info");
  const [title, setTitle] = useState(KINDS[0].title);
  const [body, setBody] = useState(KINDS[0].body);
  const [path, setPath] = useState("");
  const [audience, setAudience] = useState<string[]>([]);
  const [email, setEmail] = useState(false);
  const [sent, setSent] = useState<{ recipients: number; mailed: number } | null>(null);

  const pick = (id: (typeof KINDS)[number]["id"]) => {
    const k = KINDS.find((x) => x.id === id)!;
    setKind(id); setSeverity(k.sev); setTitle(k.title); setBody(k.body); setSent(null);
  };
  const toggle = (p: string) => setAudience((a) => (a.includes(p) ? a.filter((x) => x !== p) : [...a, p]));
  const valid = title.trim().length >= 3 && body.length <= 2000 && (path === "" || path.startsWith("/"));
  const submit = () => send.mutate({ kind, severity, title: title.trim(), body: body.trim(), path: path.trim(), audience_plans: audience, email_copy: email },
    { onSuccess: (r) => setSent({ recipients: r.recipients, mailed: r.mailed_organizations }) });

  return (
    <div className="space-y-5">
      <PageTitle title="Announcements" sub="A message to the people of every organization (or of the plans you choose): a new release, planned maintenance, a notice or an incident. It appears in their bell and on their Notifications page, and optionally by e-mail to owners and admins." />
      <Card index={0} interactive={false}>
        <CardHeader icon={<IconSend className="size-[18px] text-leaf-deep" />} title="New announcement" />
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" role="radiogroup" aria-label="Kind">
          {KINDS.map((k) => (
            <button key={k.id} type="button" role="radio" aria-checked={kind === k.id} onClick={() => pick(k.id)}
              className={cn("flex items-center gap-3 rounded-2xl border p-3 text-left transition-all hover:-translate-y-0.5", kind === k.id ? "border-leaf bg-leaf-soft/60 shadow-[0_0_0_3px_rgba(111,195,168,0.18)]" : "border-line bg-white hover:border-leaf/50")}>
              <k.icon className={cn("size-9 shrink-0", k.tone)} strokeWidth={1.4} />
              <span className="text-sm font-semibold capitalize">{k.id === "update" ? "Platform update" : k.id}</span>
            </button>
          ))}
        </div>
        <div className="mt-5 grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
          <div className="space-y-3">
            <label className="block text-xs font-medium text-slate-500">Title<input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={200} className={cn(field, "mt-1")} placeholder="What organizations should know" /></label>
            <label className="block text-xs font-medium text-slate-500">Message<textarea value={body} onChange={(e) => setBody(e.target.value)} maxLength={2000} rows={5} className={cn(field, "mt-1")} placeholder="A few clear sentences. What changes, when, and what they need to do." /></label>
            <label className="block text-xs font-medium text-slate-500">Link inside the app (optional)<input value={path} onChange={(e) => setPath(e.target.value)} maxLength={200} className={cn(field, "mt-1")} placeholder="/settings#plan" /></label>
            {path !== "" && !path.startsWith("/") && <p role="alert" className="text-xs text-bad">The link must start with /.</p>}
          </div>
          <div className="space-y-4">
            <div>
              <p className="mb-1.5 text-xs font-medium text-slate-500">Importance</p>
              <div className="flex gap-1 rounded-xl bg-slate-100 p-1 text-sm">
                {(["info", "warning", "critical"] as const).map((s) => <button key={s} type="button" aria-pressed={severity === s} onClick={() => setSeverity(s)} className={cn("flex-1 rounded-lg px-3 py-1.5 font-medium capitalize transition-all", severity === s ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink")}>{s}</button>)}
              </div>
            </div>
            <div>
              <p className="mb-1.5 text-xs font-medium text-slate-500">Who gets it</p>
              <div className="flex flex-wrap gap-1.5">
                <button type="button" onClick={() => setAudience([])} aria-pressed={audience.length === 0} className={cn("rounded-full px-3 py-1 text-xs font-semibold transition-all", audience.length === 0 ? "bg-leaf-deep text-white" : "bg-slate-100 text-slate-600 hover:bg-leaf-soft")}>Everyone</button>
                {(plans.data ?? []).map((p) => <button key={p.id} type="button" onClick={() => toggle(p.id)} aria-pressed={audience.includes(p.id)} className={cn("rounded-full px-3 py-1 text-xs font-semibold transition-all", audience.includes(p.id) ? "bg-leaf-deep text-white" : "bg-slate-100 text-slate-600 hover:bg-leaf-soft")}>{p.name}</button>)}
              </div>
            </div>
            <label className="flex cursor-pointer items-start gap-2.5 text-sm text-slate-700">
              <input type="checkbox" className="peer sr-only" checked={email} onChange={(e) => setEmail(e.target.checked)} />
              <span aria-hidden className="mt-0.5 grid size-[18px] shrink-0 place-items-center rounded-md border border-slate-300 bg-white transition-all peer-checked:border-leaf peer-checked:bg-leaf"><IconMail className={cn("size-3 text-white transition-all", email ? "scale-100 opacity-100" : "scale-50 opacity-0")} /></span>
              <span>Also e-mail owners and admins<span className="block text-xs text-muted">Sent once, cannot be recalled. Needs e-mail to be set up.</span></span>
            </label>
            <div className="rounded-2xl bg-slate-50 p-3 text-xs text-slate-600">
              <p className="mb-1 font-semibold uppercase tracking-wide text-slate-500">Preview</p>
              <p className="flex items-center gap-2 text-sm font-semibold text-ink">{title || "Title"}<span className={cn("rounded-full px-2 py-0.5 text-[10px] font-bold uppercase", SEV_CHIP[severity])}>{severity}</span></p>
              <p className="mt-0.5 line-clamp-3 whitespace-pre-line text-slate-600">{body || "Message"}</p>
            </div>
          </div>
        </div>
        <div className="mt-5 flex flex-wrap items-center gap-3 border-t border-line/70 pt-4">
          {canAct ? (
            <ConfirmButton tone="primary" label="Send announcement" confirmLabel={`Send to ${audience.length === 0 ? "everyone" : audience.join(", ")}`} disabled={!valid || send.isPending} onConfirm={submit} />
          ) : <p className="text-sm text-muted">Only an operator with the admin role can send announcements.</p>}
          {sent && <span role="status" className="text-sm text-leaf-deep">Sent to {sent.recipients} organization{sent.recipients === 1 ? "" : "s"}{sent.mailed > 0 ? `, e-mailed in ${sent.mailed}` : ""}.</span>}
          {send.isError && <span role="alert" className="text-sm text-bad">{send.error.message}</span>}
        </div>
      </Card>

      <Card index={1} interactive={false}>
        <CardHeader title="Sent" />
        {list.isLoading ? <Skeleton className="h-24" /> : list.isError ? <ErrorState /> : (list.data?.items.length ?? 0) === 0 ? <EmptyState icon={IconSend} text="Nothing has been announced yet." /> : (
          <ul className="divide-y divide-line/70">
            {list.data!.items.map((a) => (
              <li key={a.id} className="flex flex-wrap items-start gap-3 py-3">
                <span className={cn("rounded-full px-2.5 py-1 text-[11px] font-bold uppercase", SEV_CHIP[a.severity])}>{a.kind}</span>
                <span className="min-w-0 flex-1 basis-64">
                  <span className={cn("block text-sm font-semibold", a.withdrawn_at && "text-slate-400 line-through")}>{a.title}</span>
                  {a.body && <span className="mt-0.5 line-clamp-2 block text-sm text-slate-600">{a.body}</span>}
                  <span className="mt-1 block text-xs text-muted">{timeAgo(a.created_at)} · {a.audience_plans.length ? a.audience_plans.join(", ") : "everyone"} · {a.recipients} organization{a.recipients === 1 ? "" : "s"} · seen in {a.read_by}{a.email_copy ? " · e-mailed" : ""}{a.withdrawn_at ? " · withdrawn" : ""}</span>
                </span>
                {canAct && !a.withdrawn_at && <ConfirmButton label="Withdraw" confirmLabel="Withdraw from inboxes" disabled={withdraw.isPending} onConfirm={() => withdraw.mutate(a.id)} />}
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
