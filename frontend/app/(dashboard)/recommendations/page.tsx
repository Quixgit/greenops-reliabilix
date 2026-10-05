import { Lightbulb } from "lucide-react";
import { ComingSoon } from "@/components/layout/coming-soon";

export const metadata = { title: "Recommendations · Reliabilix GreenOps" };

export default function Page() {
  return <ComingSoon title="Recommendations" icon={Lightbulb} text="Available in the next version. Recommendations will appear here once we've analyzed your infrastructure." />;
}
