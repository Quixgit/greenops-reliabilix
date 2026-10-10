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
    <main className="relative flex min-h-screen items-center justify-center overflow-hidden p-5 sm:p-8">
      <LoginBackdrop />
      <div className="sign-in-rise relative z-10 w-full max-w-[400px]">
        <div className="sign-in-card rounded-2xl p-7 sm:p-9">
          <div className="mb-7 flex items-center gap-3">
            <Logo className="size-10" />
            <span className="leading-tight">
              <span className="block text-lg font-bold tracking-tight text-slate-900">Reliabilix</span>
              <span className="block text-xs font-medium text-emerald-700">GreenOps Platform</span>
            </span>
          </div>
          <LoginForm mode={dev ? "dev" : "auth0"} returnTo={returnTo} devLogin={devLoginEnabled()} error={q.error} />
        </div>
      </div>
      <p className="absolute inset-x-0 bottom-4 z-10 px-4 text-center text-[11px] text-white/75 [text-shadow:0_1px_2px_rgba(0,0,0,0.5)]">
        Photos: Pierre Blaché and Zed Can77 (CC0, Wikimedia Commons)
      </p>
    </main>
  );
}
