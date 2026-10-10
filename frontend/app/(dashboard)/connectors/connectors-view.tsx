"use client";

import Link from "next/link";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { useConnections } from "@/hooks/use-overview";
import { CATALOG, CATEGORIES, type CatalogItem } from "./catalog";

function Action({ item, connected, canConnect }: { item: CatalogItem; connected: number; canConnect: boolean }) {
  if (item.status === "planned") return <Badge tone="stub">{item.phase ?? "Planned"}</Badge>;
  if (item.status === "platform") return <Badge tone="neutral">Managed by Reliabilix</Badge>;
  const href = item.category === "notifications" ? "/settings" : "/cloud";
  if (!canConnect) return <span className="text-xs text-muted">{connected ? `${connected} connected` : "Not connected"}</span>;
  return <Link href={href} className="text-sm font-medium text-primary hover:underline">{connected ? "Manage" : "Connect"}</Link>;
}

export function ConnectorsView({ canConnect }: { canConnect: boolean }) {
  const [cat, setCat] = useState<(typeof CATEGORIES)[number]["id"]>("all");
  const conns = useConnections();
  const count = (provider?: string) => (provider ? (conns.data ?? []).filter((c) => c.provider === provider).length : 0);
  const items = CATALOG.filter((i) => cat === "all" || i.category === cat);
  const connected = (conns.data ?? []).length;
  const attention = (conns.data ?? []).filter((c) => c.sync_status === "error").length;
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Connectors</h1>
      <div className="grid gap-4 sm:grid-cols-3">
        <Card index={0}><p className="text-sm text-muted">Connected</p><p className="text-2xl font-bold">{connected}</p></Card>
        <Card index={1}><p className="text-sm text-muted">Needs attention</p><p className="text-2xl font-bold">{attention}</p></Card>
        <Card index={2}><p className="text-sm text-muted">Available now</p><p className="text-2xl font-bold">{CATALOG.filter((i) => i.status === "available").length}</p></Card>
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
              <Badge tone={item.status === "available" ? "good" : "neutral"}>{item.status === "available" ? "Available" : item.status === "platform" ? "Active" : "Not yet"}</Badge>
              <Action item={item} connected={count(item.provider)} canConnect={canConnect} />
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}
