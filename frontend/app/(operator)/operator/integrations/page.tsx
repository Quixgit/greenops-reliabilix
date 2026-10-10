import { redirect } from "next/navigation";

// Integrations moved into Data sources: every source and service has its settings on its own page.
export default function Page() {
  redirect("/operator/data-sources");
}
