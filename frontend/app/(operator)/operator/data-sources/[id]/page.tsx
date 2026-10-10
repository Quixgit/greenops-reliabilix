import { SourceDetailView } from "@/components/operator/source-detail";

export const metadata = { title: "Data source · Operator console" };

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <SourceDetailView id={id} />;
}
