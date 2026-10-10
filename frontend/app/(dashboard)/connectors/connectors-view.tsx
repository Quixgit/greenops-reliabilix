"use client";

import { Cloud } from "lucide-react";
import Link from "next/link";
import { useRef, useState } from "react";
import { ConnectionForm } from "@/components/forms/connection-form";
import { Badge, statusTone } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useConnections } from "@/hooks/use-overview";
import { timeAgo } from "@/lib/formatters";
import { CATALOG, CATEGORIES, type CatalogItem } from "./catalog";

type Provider = "aws" | "gcp";

function Action({ item, connected, canConnect, onConnect }: { item: CatalogItem; connected: number; canConnect: boolean; onConnect: (p: Provider) => void }) {
  if (item.status === "planned") return <Badge tone="stub">{item.phase ?? "Planned"}</Badge>;
  if (item.status === "platform") return <Badge tone="neutral">Managed by Reliabilix</Badge>;
  if (item.category === "cloud" && (item.provider === "aws" || item.provider === "gcp")) {
    const p = item.provider;
    return canConnect
      ? <button onClick={() => onConnect(p)} className="text-sm font-medium text-primary hover:underline">{connected ? "Add another" : "Connect"}</button>
      : <span className="text-xs text-muted">{connected ? `${connected} connected` : "Not connected"}</span>;
  }
  if (item.category === "notifications") return canConnect ? <Link href="/settings" className="text-sm font-medium text-primary hover:underline">Set up</Link> : <span className="text-xs text-muted">Available</span>;
  return null;
}

/** Cloud accounts and every other integration in one place: connected accounts and the connect flow on top, the catalog below. */
export function ConnectorsView({ canConnect }: { canConnect: boolean }) {
  const [cat, setCat] = useState<(typeof CATEGORIES)[number]["id"]>("all");
  const [connectAs, setConnectAs] = useState<Provider | null>(null);
  const accounts = useRef<HTMLDivElement>(null);
  const conns = useConnections();
  const count = (provider?: string) => (provider ? (conns.data ?? []).filter((c) => c.provider === provider).length : 0);
  const items = CATALOG.filter((i) => cat === "all" || i.category === cat);
  const connected = (conns.data ?? []).length;
  const attention = (conns.data ?? []).filter((c) => c.sync_status === "error").length;
  const open = (p: Provider) => { setConnectAs(p); setTimeout(() => accounts.current?.scrollIntoView({ behavior: "smooth", block: "start" }), 50); };
  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Connectors</h1>
        <p className="mt-1 text-sm text-muted">Your cloud accounts and every integration of the platform.</p>
      </div>
      <div className="grid gap-4 sm:grid-cols-3">
        <Card index={0}><p className="text-sm text-muted">Cloud accounts</p><p className="text-2xl font-bold">{connected}</p></Card>
        <Card index={1}><p className="text-sm text-muted">Needs attention</p><p className="text-2xl font-bold">{attention}</p></Card>
        <Card index={2}><p className="text-sm text-muted">Available now</p><p className="text-2xl font-bold">{CATALOG.filter((i) => i.status === "available").length}</p></Card>
      </div>

      <div ref={accounts} id="accounts" className="scroll-mt-20">
        <Card index={3}>
          <CardHeader title="Cloud accounts" action={canConnect && !connectAs ? <button onClick={() => setConnectAs("aws")} className="rounded-lg px-3 py-1.5 text-sm font-semibold text-primary ring-1 ring-line hover:bg-slate-50">Connect an account</button> : undefined} />
          {connectAs && canConnect && (
            <div className="mb-5 rounded-xl bg-slate-50 p-4 ring-1 ring-line">
              <ConnectionForm key={connectAs} initialProvider={connectAs} onDone={() => setConnectAs(null)} />
              <button onClick={() => setConnectAs(null)} className="mt-3 text-sm text-muted hover:text-ink">Cancel</button>
            </div>
          )}
          {conns.isLoading ? <Skeleton className="h-20" /> : conns.isError ? <ErrorState /> : conns.data!.length === 0 ? (
            <EmptyState icon={Cloud} text="No cloud accounts connected yet. Connect AWS or Google Cloud to see cost and carbon." />
          ) : (
            <ul className="divide-y divide-line/70 text-sm">
              {conns.data!.map((c) => (
                <li key={c.id} className="flex flex-wrap items-center justify-between gap-2 py-2.5">
                  <Link href={`/cloud/${c.id}`} className="hover:underline"><b className="uppercase">{c.provider}</b> <span className="text-muted">{c.account_ref}</span></Link>
                  <span className="flex items-center gap-2 text-xs text-muted">
                    <Badge tone={statusTone(c.sync_status)}>{c.sync_status}</Badge>
                    {c.last_sync_at ? `synced ${timeAgo(c.last_sync_at)}` : "not synced yet"}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>

      <div role="tablist" className="flex flex-wrap gap-2">
        {CATEGORIES.map((c) => (
          <button key={c.id} role="tab" aria-selected={cat === c.id} onClick={() => setCat(c.id)}
            className={`rounded-full px-3 py-1 text-sm ring-1 ring-inset ${cat === c.id ? "bg-primary text-white ring-primary" : "bg-white text-slate-700 ring-line"}`}>
            {c.label}
          </button>
        ))}
      </div>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {items.map((item, i) => (
          <Card key={item.id} index={i} className="flex flex-col justify-between gap-3">
            <div>
              <p className="font-semibold">{item.name}</p>
              <p className="mt-1 text-sm text-muted">{item.provides}</p>
            </div>
            <div className="flex items-center justify-between">
              <Badge tone={item.status === "available" ? "good" : "neutral"}>{item.status === "available" ? (count(item.provider) ? `${count(item.provider)} connected` : "Available") : item.status === "platform" ? "Active" : "Not yet"}</Badge>
              <Action item={item} connected={count(item.provider)} canConnect={canConnect} onConnect={open} />
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}
