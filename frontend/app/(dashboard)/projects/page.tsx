import { ProjectsView } from "./projects-view";
import { getUser } from "@/lib/auth/session";
import { canWrite } from "@/lib/permissions";

export const metadata = { title: "Projects · Reliabilix GreenOps" };

export default async function ProjectsPage() {
  const user = (await getUser())!;
  return <ProjectsView canCreate={canWrite(user.role)} />;
}
