import { FileText } from "lucide-react";
import { ComingSoon } from "@/components/layout/coming-soon";

export const metadata = { title: "Reports · Reliabilix GreenOps" };

export default function Page() {
  return <ComingSoon title="Reports" icon={FileText} text="Available in the next version. PDF and CSV carbon reports will be listed here." />;
}
