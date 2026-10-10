import { redirect } from "next/navigation";
import { Logo } from "@/components/layout/logo";
import { LoginArt } from "@/components/auth/login-art";
import { LoginForm } from "@/components/auth/login-form";
import { devAuthEnabled } from "@/lib/auth/dev";
import { devLoginEnabled } from "@/lib/auth/devsession";
import { safeReturnTo } from "@/lib/auth/login";
import { getUser } from "@/lib/auth/session";

export const dynamic = "force-dynamic";
export const metadata = { title: "Sign in · Reliabilix GreenOps" };

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ returnTo?: string; preview?: string; error?: string }> }) {
  const q = await searchParams;
  const dev = devAuthEnabled();
  // In the development environment everybody is signed in already; ?preview shows the page anyway for design review.
  if (!(dev && q.preview !== undefined) && (await getUser())) redirect(safeReturnTo(q.returnTo));
  const returnTo = safeReturnTo(q.returnTo);
  return (
    <main className="grid min-h-screen bg-white lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)]">
      <section className="relative hidden overflow-hidden lg:block">
        <LoginArt className="absolute inset-0 h-full w-full" />
        <div className="relative z-10 flex h-full flex-col p-10">
          <div className="flex items-center gap-3">
            <Logo className="size-10" />
            <span className="leading-tight"><span className="block text-xl font-bold tracking-tight text-ink">Reliabilix</span><span className="block text-xs text-slate-600">GreenOps Platform</span></span>
          </div>
          <div className="mt-14 max-w-md">
            <h2 className="text-[32px] font-bold leading-tight tracking-tight text-ink">Cloud cost and carbon, one clear picture.</h2>
            <p className="mt-3 text-[15px] leading-relaxed text-slate-700">See what your cloud costs and how much CO₂e it causes, and get safe, human-approved ways to reduce both.</p>
          </div>
        </div>
      </section>
      <section className="flex items-center justify-center p-6 sm:p-10">
        <div className="w-full max-w-[400px]">
          <div className="mb-8 flex items-center gap-3 lg:hidden">
            <Logo className="size-9" />
            <span className="text-lg font-bold tracking-tight">Reliabilix <span className="font-medium text-muted">GreenOps</span></span>
          </div>
          <div className="glass p-7 sm:p-8">
            <LoginForm mode={dev ? "dev" : "auth0"} returnTo={returnTo} devLogin={devLoginEnabled()} error={q.error} />
          </div>
          <p className="mt-5 text-center text-xs text-muted">© Reliabilix · <span>Secure sign-in</span></p>
        </div>
      </section>
    </main>
  );
}
