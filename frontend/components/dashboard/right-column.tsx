import { ArrowRight, Lightbulb } from "lucide-react";
import Link from "next/link";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { content } from "@/content/schema";

export function PromoBanner() {
  const b = content.promoBanner;
  return (
    <Card className="relative overflow-hidden p-0">
      <div className="absolute inset-0 bg-cover bg-center" style={{ backgroundImage: `url(${b.imageUrl})` }} aria-hidden />
      <div className="absolute inset-0 bg-gradient-to-t from-emerald-950/80 via-emerald-950/30 to-transparent" aria-hidden />
      <div className="relative flex min-h-56 flex-col justify-end gap-2 p-5 text-white">
        <h2 className="text-lg font-bold">{b.title}</h2>
        <p className="text-sm text-white/85">{b.text}</p>
        <Link href="/recommendations" className="mt-1 inline-flex w-fit items-center gap-1.5 rounded-lg bg-white px-3.5 py-2 text-sm font-semibold text-emerald-900 hover:bg-emerald-50">
          {b.cta} <ArrowRight className="size-4" />
        </Link>
      </div>
    </Card>
  );
}

/** Phase 1: the recommendations domain does not exist yet, so this panel is deliberately empty (no invented savings). */
export function TopRecommendations() {
  return (
    <Card>
      <CardHeader title="Top Recommendations" />
      <EmptyState icon={Lightbulb} text={content.recommendationsEmpty} />
    </Card>
  );
}

export const Tagline = () => <p className="pt-2 text-center text-sm text-muted">{content.tagline}</p>;
