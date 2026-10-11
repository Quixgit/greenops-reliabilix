import { CiCdView } from "./ci-cd-view";
import { getUser } from "@/lib/auth/session";
import { canManageTenant } from "@/lib/permissions";

export const metadata = { title: "CI/CD · Reliabilix GreenOps" };

export default async function Page() {
  const user = (await getUser())!;
  return <CiCdView canEdit={canManageTenant(user.role)} />;
}
