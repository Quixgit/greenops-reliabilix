import { ReportsView } from "./reports-view";
import { getUser } from "@/lib/auth/session";
import { canWrite } from "@/lib/permissions";

export const metadata = { title: "Reports · Reliabilix GreenOps" };

export default async function Page() {
  const user = (await getUser())!;
  return <ReportsView canCreate={canWrite(user.role)} />;
}
