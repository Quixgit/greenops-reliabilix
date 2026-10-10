import { ConnectorsView } from "./connectors-view";
import { getUser } from "@/lib/auth/session";
import { canWrite } from "@/lib/permissions";

export const metadata = { title: "Connectors · Reliabilix GreenOps" };

export default async function Page() {
  const user = (await getUser())!;
  return <ConnectorsView canConnect={canWrite(user.role)} />;
}
