import { Card, CardHeader } from "@/components/ui/card";
import { getUser } from "@/lib/auth/session";

export const metadata = { title: "Settings · Reliabilix GreenOps" };

export default async function SettingsPage() {
  const user = (await getUser())!;
  return (
    <div className="max-w-xl space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
      <Card>
        <CardHeader title="Profile" />
        <dl className="grid grid-cols-[120px_1fr] gap-y-2 text-sm">
          <dt className="text-muted">Name</dt><dd>{user.name}</dd>
          <dt className="text-muted">Email</dt><dd>{user.email || "—"}</dd>
          <dt className="text-muted">Role</dt><dd className="capitalize">{user.role}</dd>
        </dl>
      </Card>
    </div>
  );
}
