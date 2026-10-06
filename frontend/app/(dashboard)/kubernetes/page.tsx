import { Boxes } from "lucide-react";
import { ComingSoon } from "@/components/layout/coming-soon";

export const metadata = { title: "Kubernetes · Reliabilix GreenOps" };

export default function Page() {
  return <ComingSoon title="Kubernetes" icon={Boxes} text="Available in the next version. Kubernetes energy data (Kepler) will appear here." />;
}
