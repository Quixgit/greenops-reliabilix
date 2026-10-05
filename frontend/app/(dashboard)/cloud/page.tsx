import { CloudView } from "./cloud-view";
import { getUser } from "@/lib/auth/session";
import { canWrite } from "@/lib/permissions";

export const metadata = { title: "Cloud Accounts · Reliabilix GreenOps" };

export default async function CloudPage() {
  const user = (await getUser())!;
  return <CloudView canConnect={canWrite(user.role)} />;
}
