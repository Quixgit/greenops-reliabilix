import { Workflow } from "lucide-react";
import { ComingSoon } from "@/components/layout/coming-soon";

export const metadata = { title: "Automation · Reliabilix GreenOps" };

export default function Page() {
  return <ComingSoon title="Automation" icon={Workflow} text="Available in a later version. Approved recommendations will be applied here, with human approval and rollback." />;
}
