import { Hourglass } from "lucide-react";
import Link from "next/link";
import { Card, CardHeader } from "@/components/ui/card";
import { PageTitle } from "./parts";
import type { PlannedSection } from "./planned";

/** A menu section that is not built yet. It never shows invented numbers: what exists today, then what is next. */
export function PlannedView({ section }: { section: PlannedSection }) {
  return (
    <div className="space-y-5">
      <PageTitle title={section.title} sub={section.summary} action={
        <span className="inline-flex items-center gap-1.5 rounded-full bg-violet-50 px-3 py-1 text-xs font-medium text-violet-700 ring-1 ring-violet-200"><Hourglass className="size-3.5" aria-hidden />Planned · {section.stage}</span>
      } />
      <div className="grid gap-4 lg:grid-cols-2">
        <Card index={0}>
          <CardHeader title="Available today" />
          <ul className="space-y-2 text-sm">
            {section.today.map((t) => (
              <li key={t.href + t.label}><Link href={t.href} className="inline-flex items-center gap-2 rounded-xl bg-leaf-soft px-3 py-2 font-medium text-primary hover:bg-leaf-soft">{t.label} →</Link></li>
            ))}
          </ul>
        </Card>
        <Card index={1}>
          <CardHeader title="What this section will hold" />
          <ol className="space-y-2 text-sm">
            {section.scope.map((s, i) => (
              <li key={s} className="flex gap-3"><span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-violet-50 text-xs font-semibold text-slate-700">{i + 1}</span><span className="pt-0.5">{s}</span></li>
            ))}
          </ol>
        </Card>
      </div>
    </div>
  );
}
