import { SettingsView } from "./settings-view";
import { getUser } from "@/lib/auth/session";

export const metadata = { title: "Settings · Reliabilix GreenOps" };

export default async function SettingsPage() {
  const user = (await getUser())!;
  return <SettingsView user={{ name: user.name, email: user.email, role: user.role }} />;
}
