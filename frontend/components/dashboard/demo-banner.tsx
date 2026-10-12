"use client";

import Link from "next/link";
import { useState } from "react";
import { IconArrowRight, IconSparkles, IconTrash } from "@/components/ui/icons";
import { useCreateDemo, useDemo, useRemoveDemo } from "@/hooks/use-demo";
import { useConnections } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cn } from "@/lib/utils";

/**
 * Two states of one idea. Without a cloud account and without a sample: an invitation to look around with sample data. With the sample:
 * a plain notice that the numbers are invented, with the way out (connect a cloud, or remove the sample).
 */
export function DemoBanner({ canWrite }: { canWrite: boolean }) {
  const t = useT();
  const demo = useDemo();
  const conns = useConnections();
  const create = useCreateDemo();
  const remove = useRemoveDemo();
  const [ask, setAsk] = useState(false);
  if (demo.isLoading || conns.isLoading) return null;
  const active = !!demo.data?.active;
  const hasCloud = (conns.data?.length ?? 0) > 0;
  if (!active && (hasCloud || !canWrite)) return null;
  const err = create.error ?? remove.error;
  return (
    <section className={cn("rise flex flex-wrap items-center justify-between gap-x-6 gap-y-3 rounded-2xl px-5 py-4 ring-1", active ? "bg-[#fdf3dc]/70 ring-[#f2cf8a]" : "bg-sky-50/70 ring-sky-100")} aria-label={t("dm.title")}>
      <div className="flex min-w-0 flex-1 basis-80 items-start gap-3">
        <span className={cn("mt-0.5 grid size-9 shrink-0 place-items-center rounded-xl", active ? "bg-[#f8e3b4] text-[#a9741a]" : "bg-white text-[#3f78b8]")}><IconSparkles className="size-[18px]" /></span>
        <div className="min-w-0">
          <p className="text-sm font-semibold text-ink">{active ? t("dm.active_title") : t("dm.title")}</p>
          <p className="mt-0.5 text-xs leading-relaxed text-slate-600">{active ? t("dm.active_body") : t("dm.body")}</p>
          {err && <p role="alert" className="mt-1 text-xs text-bad">{err.message}</p>}
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        {!active ? (
          <button type="button" disabled={create.isPending} onClick={() => create.mutate()} className="btn-leaf inline-flex items-center gap-2 rounded-xl px-4 py-2 text-sm font-semibold">
            <IconSparkles className="size-4" />{create.isPending ? t("dm.creating") : t("dm.try")}
          </button>
        ) : (
          <>
            <Link href="/connectors" className="btn-leaf inline-flex items-center gap-2 rounded-xl px-4 py-2 text-sm font-semibold">{t("dm.connect")}<IconArrowRight className="size-4" /></Link>
            {canWrite && (ask ? (
              <span className="form-swap flex items-center gap-2">
                <button type="button" disabled={remove.isPending} onClick={() => remove.mutate()} className="rounded-xl bg-[#d9707f] px-3 py-2 text-xs font-semibold text-white">{remove.isPending ? t("dm.removing") : t("dm.remove_yes")}</button>
                <button type="button" onClick={() => setAsk(false)} className="rounded-xl px-3 py-2 text-xs font-semibold text-slate-600 hover:bg-white">{t("rs.cancel")}</button>
              </span>
            ) : (
              <button type="button" onClick={() => setAsk(true)} className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs font-semibold text-slate-600 transition-colors hover:bg-white hover:text-[#c25667]"><IconTrash className="size-3.5" />{t("dm.remove")}</button>
            ))}
          </>
        )}
      </div>
    </section>
  );
}
