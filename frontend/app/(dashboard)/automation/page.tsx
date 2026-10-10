import { AutomationView } from "./automation-view";
import { getUser } from "@/lib/auth/session";
import { canAutomate, canSeeAutomation } from "@/lib/permissions";

export const metadata = { title: "Automation · Reliabilix GreenOps" };

export default async function Page() {
  const user = (await getUser())!;
  return <AutomationView canSee={canSeeAutomation(user.role)} canAct={canAutomate(user.role)} />;
}
