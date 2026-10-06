import type { LucideIcon } from "lucide-react";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";

export function ComingSoon({ title, icon, text }: { title: string; icon: LucideIcon; text: string }) {
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
      <Card><EmptyState icon={icon} text={text} className="py-16" /></Card>
    </div>
  );
}
