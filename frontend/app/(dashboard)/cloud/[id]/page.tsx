import { AccountView } from "./account-view";
import { getUser } from "@/lib/auth/session";
import { canWrite } from "@/lib/permissions";

export const metadata = { title: "Cloud account · Reliabilix GreenOps" };

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const [{ id }, user] = await Promise.all([params, getUser()]);
  return <AccountView id={id} canWrite={canWrite(user!.role)} />;
}
