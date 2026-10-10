import { AlertsView } from "./alerts-view";
import { getUser } from "@/lib/auth/session";

export const metadata = { title: "Alerts · Reliabilix GreenOps" };

export default async function Page() {
  const user = (await getUser())!;
  return <AlertsView canAct={user.role === "owner" || user.role === "admin" || user.role === "engineer"} />;
}
