import { IconBranch as GitBranch } from "@/components/ui/icons";
import { ComingSoon } from "@/components/layout/coming-soon";

export const metadata = { title: "CI/CD · Reliabilix GreenOps" };

export default function Page() {
  return (
    <ComingSoon
      title="CI/CD"
      icon={GitBranch}
      text="Pipeline runs, builds and their carbon estimate arrive with the GitHub Actions connector (phase 3). Today a pipeline can already ask the CI gate (POST /ci/evaluate with an API key) whether a deploy fits the project's carbon policy."
    />
  );
}
