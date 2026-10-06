import { redirect } from "next/navigation";
import { Logo } from "@/components/layout/logo";
import { getUser } from "@/lib/auth/session";

export const dynamic = "force-dynamic";

export default async function LoginPage() {
  if (await getUser()) redirect("/overview");
  return (
    <main className="flex min-h-screen items-center justify-center p-6">
      <div className="glass w-full max-w-sm space-y-6 p-8 text-center">
        <Logo className="mx-auto size-14" />
        <div>
          <h1 className="text-xl font-bold">Reliabilix GreenOps</h1>
          <p className="mt-1 text-sm text-muted">Sign in to see your cloud cost and carbon footprint.</p>
        </div>
        {/* A plain anchor on purpose: /auth/login is handled by the Auth0 proxy, not by the router. */}
        <a href="/auth/login?returnTo=/overview" className="brand-gradient block rounded-xl px-4 py-3 text-sm font-semibold text-white shadow-lg shadow-indigo-500/25 hover:opacity-95">
          Sign in
        </a>
      </div>
    </main>
  );
}
