import { redirect } from "next/navigation";
import { IconChart as BarChart3, IconFlow as Workflow, IconLeaf as Leaf, IconShield as ShieldCheck } from "@/components/ui/icons";
import { LoginBackdrop } from "@/components/auth/login-backdrop";
import { LoginForm } from "@/components/auth/login-form";
import { Logo } from "@/components/layout/logo";
import { devAuthEnabled } from "@/lib/auth/dev";
import { devLoginEnabled } from "@/lib/auth/devsession";
import { safeReturnTo } from "@/lib/auth/login";
import { getUser } from "@/lib/auth/session";

export const dynamic = "force-dynamic";
export const metadata = { title: "Sign in · Reliabilix GreenOps" };

const POINTS = [
  { icon: BarChart3, title: "Cost and carbon, side by side", text: "Every service, region and workload on one bill and one footprint." },
  { icon: Leaf, title: "Measured against the grid", text: "Emissions from real grid intensity, with the basis shown, not hidden." },
  { icon: Workflow, title: "From insight to action", text: "Region moves, rightsizing and time shifting, approved by people." },
  { icon: ShieldCheck, title: "Built to be trusted", text: "Isolated per organization; secrets sealed; every change on the record." },
];

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ returnTo?: string; preview?: string; error?: string }> }) {
  const q = await searchParams;
  const dev = devAuthEnabled();
  // In the development environment without the sign-in page everybody is signed in already; ?preview shows it anyway.
  if (!(dev && q.preview !== undefined) && (await getUser())) redirect(safeReturnTo(q.returnTo));
  const returnTo = safeReturnTo(q.returnTo);
  return (
    <main className="relative min-h-screen overflow-hidden text-slate-900">
      <LoginBackdrop />
      <div className="relative z-10 mx-auto grid min-h-screen max-w-[1240px] items-center gap-12 px-6 py-10 lg:grid-cols-[minmax(0,1fr)_420px] lg:px-10">
        <section className="hidden lg:block">
          <div className="login-reveal flex items-center gap-3.5" style={{ "--i": 0 } as React.CSSProperties}>
            <Logo className="h-14 w-11" />
            <span className="leading-tight">
              <span className="block text-2xl font-bold tracking-tight text-slate-900">Reliabilix</span>
              <span className="block text-xs font-medium uppercase tracking-[0.22em] text-teal-700">Systems</span>
            </span>
          </div>
          <h1 className="login-reveal mt-10 max-w-xl text-[44px] font-bold leading-[1.08] tracking-tight" style={{ "--i": 1 } as React.CSSProperties}>
            A greener cloud.<br /><span className="bg-gradient-to-r from-emerald-600 via-teal-600 to-sky-600 bg-clip-text text-transparent">A smarter bill.</span>
          </h1>
          <p className="login-reveal mt-4 max-w-lg text-[17px] leading-relaxed text-slate-600" style={{ "--i": 2 } as React.CSSProperties}>
            GreenOps and FinOps in one place: see what your cloud costs and what it emits, then do something about both.
          </p>
          <ul className="mt-9 grid max-w-xl gap-4 sm:grid-cols-2">
            {POINTS.map((p, i) => (
              <li key={p.title} className="login-reveal flex gap-3" style={{ "--i": i + 3 } as React.CSSProperties}>
                <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-xl bg-white/80 text-teal-700 shadow-sm ring-1 ring-slate-200 backdrop-blur"><p.icon className="size-[18px]" strokeWidth={1.8} /></span>
                <span>
                  <span className="block text-sm font-semibold text-slate-900">{p.title}</span>
                  <span className="mt-0.5 block text-[13px] leading-snug text-slate-500">{p.text}</span>
                </span>
              </li>
            ))}
          </ul>
        </section>

        <div className="login-card-in w-full lg:justify-self-end">
          <div className="mb-6 flex items-center gap-3 lg:hidden">
            <Logo className="h-11 w-9" />
            <span className="leading-tight"><span className="block text-xl font-bold text-slate-900">Reliabilix</span><span className="block text-xs font-medium uppercase tracking-[0.2em] text-teal-700">Systems</span></span>
          </div>
          <div className="login-card rounded-3xl p-7 sm:p-9">
            <LoginForm mode={dev ? "dev" : "auth0"} returnTo={returnTo} devLogin={devLoginEnabled()} error={q.error} />
          </div>
          <p className="mt-5 text-center text-[11px] text-slate-400">GreenOps Platform · Reliabilix Systems</p>
        </div>
      </div>
    </main>
  );
}
