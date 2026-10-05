import { redirect } from "next/navigation";
import { Logo } from "@/components/layout/logo";
import { OnboardingForm } from "@/components/forms/onboarding-form";
import { checkMembership } from "@/lib/api/server";
import { getUser } from "@/lib/auth/session";

export const dynamic = "force-dynamic";

export default async function OnboardingPage() {
  const user = await getUser();
  if (!user) redirect("/login");
  if ((await checkMembership()) === "member") redirect("/overview");
  return (
    <main className="flex min-h-screen items-center justify-center p-6">
      <div className="glass w-full max-w-md space-y-5 p-8">
        <Logo className="size-12" />
        <div>
          <h1 className="text-xl font-bold">Welcome, {user.name}</h1>
          <p className="mt-1 text-sm text-muted">Name your organization to get started. You will be its owner.</p>
        </div>
        <OnboardingForm />
      </div>
    </main>
  );
}
