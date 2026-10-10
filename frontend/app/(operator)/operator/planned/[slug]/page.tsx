import { notFound } from "next/navigation";
import { PlannedView } from "@/components/operator/planned-view";
import { PLANNED } from "@/components/operator/planned";

export function generateStaticParams() {
  return Object.keys(PLANNED).map((slug) => ({ slug }));
}

export default async function Page({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const section = PLANNED[slug];
  if (!section) notFound();
  return <PlannedView section={section} />;
}
