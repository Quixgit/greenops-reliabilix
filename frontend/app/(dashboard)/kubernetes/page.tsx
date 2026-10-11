import { KubernetesView } from "./kubernetes-view";
import { getUser } from "@/lib/auth/session";
import { canManageTenant } from "@/lib/permissions";

export const metadata = { title: "Kubernetes · Reliabilix GreenOps" };

export default async function Page() {
  const user = (await getUser())!;
  return <KubernetesView canConnect={canManageTenant(user.role)} />;
}
