"use client";

import { ClipboardList } from "lucide-react";
import { useState } from "react";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuditLog } from "@/hooks/use-admin";
import { useT } from "@/i18n/provider";
import { shortTime } from "@/lib/format-time";
import { cn } from "@/lib/utils";
import { Panel } from "./organization-tab";

const AREAS = ["all", "member", "cloud_connection", "api_key", "recommendation", "setting", "organization"] as const;

/** The organization's audit trail: who did what and when. Append-only on the server; read-only here. */
export function AuditPanel() {
  const t = useT();
  const q = useAuditLog(200);
  const [area, setArea] = useState<(typeof AREAS)[number]>("all");
  const filtered = (q.data ?? []).filter((r) => area === "all" || r.action.startsWith(area));
  const pg = usePaged(filtered, 10);
  const rows = pg.slice;
  return (
    <Panel icon={ClipboardList} title={t("settings.audit.title")} hint={t("settings.audit.hint")}>
      <div role="tablist" className="mb-3 flex flex-wrap gap-1.5">
        {AREAS.map((a) => (
          <button key={a} role="tab" aria-selected={area === a} type="button" onClick={() => { setArea(a); pg.setPage(1); }}
            className={cn("rounded-full px-3 py-1 text-xs font-medium ring-1 ring-inset transition-all duration-200",
              area === a ? "bg-teal-700 text-white ring-teal-700" : "bg-white text-slate-600 ring-line hover:bg-slate-50")}>
            {t(`settings.audit.area.${a}`)}
          </button>
        ))}
      </div>
      {q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> : rows.length === 0 ? <EmptyState icon={ClipboardList} text={t("settings.audit.none")} /> : (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="text-xs text-muted"><tr><th className="py-2 pr-3 font-medium">{t("settings.audit.when")}</th><th className="pr-3 font-medium">{t("settings.audit.who")}</th><th className="pr-3 font-medium">{t("settings.audit.what")}</th><th className="font-medium">{t("settings.audit.target")}</th></tr></thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.id} className="border-t border-line/70 transition-colors hover:bg-slate-50/70">
                  <td className="whitespace-nowrap py-2 pr-3 text-muted">{shortTime(r.created_at)}</td>
                  <td className="max-w-[10rem] truncate pr-3">{r.actor_user_id.startsWith("apikey:") ? t("settings.audit.api_key") : r.actor_user_id}</td>
                  <td className="pr-3"><code className="rounded bg-slate-100 px-1.5 py-0.5 text-xs">{r.action}</code></td>
                  <td className="max-w-[16rem] truncate text-xs text-muted" title={r.target}>{r.target}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
    </Panel>
  );
}
