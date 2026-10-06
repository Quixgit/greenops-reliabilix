import { RecommendationsView } from "./recommendations-view";
import { getUser } from "@/lib/auth/session";
import { canWrite } from "@/lib/permissions";

export const metadata = { title: "Recommendations · Reliabilix GreenOps" };

export default async function Page() {
  const user = (await getUser())!;
  return <RecommendationsView canDecide={canWrite(user.role)} />;
}
