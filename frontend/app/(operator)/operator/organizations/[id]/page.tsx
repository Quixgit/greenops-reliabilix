import { notFound } from "next/navigation";
import { OrganizationView } from "@/components/operator/org-detail";

export const metadata = { title: "Organization · Operator console" };

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!UUID.test(id)) notFound();
  return <OrganizationView id={id} />;
}
