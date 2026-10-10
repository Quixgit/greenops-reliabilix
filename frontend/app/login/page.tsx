import { redirect } from "next/navigation";
import { LoginBackdrop } from "@/components/auth/login-backdrop";
import { LoginForm } from "@/components/auth/login-form";
import { Logo } from "@/components/layout/logo";
import { devAuthEnabled } from "@/lib/auth/dev";
import { devLoginEnabled } from "@/lib/auth/devsession";
import { safeReturnTo } from "@/lib/auth/login";
import { getUser } from "@/lib/auth/session";

export const dynamic = "force-dynamic";
export const metadata = { title: "Sign in · Reliabilix GreenOps" };

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ returnTo?: string; preview?: string; error?: string }> }) {
  const q = await searchParams;
  const dev = devAuthEnabled();
  // In the development environment without the sign-in page everybody is signed in already; ?preview shows it anyway.
  if (!(dev && q.preview !== undefined) && (await getUser())) redirect(safeReturnTo(q.returnTo));
  const returnTo = safeReturnTo(q.returnTo);
  return (
    <main className="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#0a1420] p-5 sm:p-8">
      <LoginBackdrop />
      <div className="ind-in relative z-10 w-full max-w-[420px]">
        <div className="mb-6 flex items-center justify-center gap-3">
          <Logo className="size-11" />
          <span className="leading-tight">
            <span className="block text-[22px] font-bold tracking-tight text-white">Reliabilix</span>
            <span className="block font-mono text-[11px] uppercase tracking-[0.2em] text-cyan-200/80">GreenOps Platform</span>
          </span>
        </div>
        <div className="ind-card rounded-lg p-7 sm:p-9">
          <LoginForm mode={dev ? "dev" : "auth0"} returnTo={returnTo} devLogin={devLoginEnabled()} error={q.error} />
        </div>
        <p className="mt-5 text-center font-mono text-[11px] uppercase tracking-[0.16em] text-slate-400">Cost · Carbon · Control</p>
      </div>
    </main>
  );
}
