"use client";

import { ArrowLeft, Eye, EyeOff, Lock, Mail, User } from "lucide-react";
import { useState } from "react";
import { loginUrl, validEmail } from "@/lib/auth/login";
import { cn } from "@/lib/utils";

function GoogleMark() {
  return (
    <svg viewBox="0 0 24 24" className="size-5" aria-hidden>
      <path fill="#4285F4" d="M23.5 12.3c0-.8-.1-1.6-.2-2.3H12v4.5h6.4a5.5 5.5 0 0 1-2.4 3.6v3h3.9c2.3-2.1 3.6-5.2 3.6-8.8z" />
      <path fill="#34A853" d="M12 24c3.2 0 6-1.1 7.9-2.9l-3.9-3c-1.1.7-2.5 1.2-4 1.2-3.1 0-5.7-2.1-6.6-4.9H1.4v3.1A12 12 0 0 0 12 24z" />
      <path fill="#FBBC05" d="M5.4 14.4a7.2 7.2 0 0 1 0-4.8V6.5H1.4a12 12 0 0 0 0 11z" />
      <path fill="#EA4335" d="M12 4.8c1.8 0 3.3.6 4.6 1.8l3.4-3.4A12 12 0 0 0 1.4 6.5l4 3.1C6.3 6.9 8.9 4.8 12 4.8z" />
    </svg>
  );
}
function GitHubMark() {
  return (
    <svg viewBox="0 0 24 24" className="size-5" aria-hidden fill="currentColor">
      <path d="M12 .5a11.5 11.5 0 0 0-3.6 22.4c.6.1.8-.3.8-.6v-2c-3.2.7-3.9-1.5-3.9-1.5-.5-1.3-1.3-1.7-1.3-1.7-1-.7.1-.7.1-.7 1.2.1 1.8 1.2 1.8 1.2 1 1.8 2.8 1.3 3.4 1 .1-.8.4-1.3.7-1.6-2.6-.3-5.3-1.3-5.3-5.7 0-1.3.5-2.3 1.2-3.1-.1-.3-.5-1.5.1-3.1 0 0 1-.3 3.2 1.2a11 11 0 0 1 5.8 0c2.2-1.5 3.2-1.2 3.2-1.2.6 1.6.2 2.8.1 3.1.8.8 1.2 1.9 1.2 3.1 0 4.4-2.7 5.4-5.3 5.7.4.4.8 1.1.8 2.2v3.2c0 .3.2.7.8.6A11.5 11.5 0 0 0 12 .5z" />
    </svg>
  );
}

const field = "block h-12 w-full rounded-xl border border-slate-200 bg-white pl-11 pr-3.5 text-sm outline-none transition-all duration-200 placeholder:text-slate-400 hover:border-slate-300 focus:border-leaf-deep focus:ring-4 focus:ring-leaf-deep/10 disabled:bg-slate-50";
const primary = "relative h-12 w-full overflow-hidden rounded-xl bg-gradient-to-b from-leaf-deep to-leaf-deep text-sm font-semibold text-white shadow-[0_10px_24px_-10px_rgba(15,118,110,0.75)] transition-all duration-200 hover:-translate-y-0.5 hover:from-leaf-deep hover:to-leaf-deep active:translate-y-0 focus:outline-none focus-visible:ring-4 focus-visible:ring-leaf-deep/30 disabled:translate-y-0 disabled:opacity-60";
const social = "flex h-12 w-full items-center justify-center gap-3 rounded-xl border border-slate-200 bg-white text-sm font-medium text-slate-800 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-md focus:outline-none focus-visible:ring-4 focus-visible:ring-leaf-deep/15 aria-disabled:pointer-events-none aria-disabled:opacity-50";

type View = "signin" | "signup" | "forgot";

function Input({ icon: Icon, ...p }: React.InputHTMLAttributes<HTMLInputElement> & { icon: typeof Mail }) {
  return (
    <div className="relative">
      <Icon className="pointer-events-none absolute left-3.5 top-1/2 size-[18px] -translate-y-1/2 text-slate-400" />
      <input {...p} className={cn(field, p.className)} />
    </div>
  );
}

function Divider({ children }: { children: string }) {
  return <div className="flex items-center gap-3 text-xs font-medium text-slate-400"><span className="h-px flex-1 bg-slate-200" />{children}<span className="h-px flex-1 bg-slate-200" /></div>;
}

/**
 * One calm form with three views that slide into one another: sign in, create an account, reset the password. In the
 * test environment the user name and password are typed here; with the identity provider the e-mail goes on and the
 * password and any second factor are only ever typed on the provider's own page.
 */
export function LoginForm({ mode, returnTo, devLogin = false, error: loginError }: { mode: "auth0" | "dev"; returnTo: string; devLogin?: boolean; error?: string }) {
  const [view, setView] = useState<View>("signin");
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [show, setShow] = useState(false);
  const [touched, setTouched] = useState(false);
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const auth0 = mode === "auth0";
  const emailOk = validEmail(email);
  const go = (v: View) => { setView(v); setTouched(false); setSent(false); setError(""); };

  async function recover(e: React.FormEvent) {
    e.preventDefault();
    setTouched(true);
    if (!emailOk) return;
    setBusy(true); setError("");
    try {
      const res = await fetch("/api/auth/forgot-password", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email }) });
      if (res.status === 429) setError("Too many attempts. Please wait a minute and try again.");
      else if (!res.ok) setError("That address does not look right.");
      else setSent(true);
    } catch {
      setError("The request could not be sent. Check your connection and try again.");
    } finally {
      setBusy(false);
    }
  }

  const back = (
    <button type="button" onClick={() => go("signin")} className="mb-5 inline-flex items-center gap-1.5 text-sm font-medium text-slate-500 transition-colors hover:text-slate-900">
      <ArrowLeft className="size-4" />Back to sign in
    </button>
  );

  const socials = (verb: string) => (
    <div className="space-y-3">
      <a href={auth0 ? loginUrl({ connection: "google-oauth2", returnTo, signup: view === "signup" }) : undefined} aria-disabled={!auth0} className={social}><GoogleMark />{verb} with Google</a>
      <a href={auth0 ? loginUrl({ connection: "github", returnTo, signup: view === "signup" }) : undefined} aria-disabled={!auth0} className={social}><GitHubMark />{verb} with GitHub</a>
      {!auth0 && <p className="text-center text-xs text-slate-400">Social sign-in needs the identity provider and is off in this test environment.</p>}
    </div>
  );

  if (view === "forgot") {
    return (
      <div key="forgot" className="form-swap">
        {back}
        <h1 className="text-[26px] font-semibold tracking-tight text-slate-900">Reset your password</h1>
        <p className="mt-1.5 text-sm text-slate-500">Enter the e-mail of your account and we will send a link to choose a new password.</p>
        <div className="mt-6">
          {sent ? (
            <p role="status" className="rounded-xl bg-leaf-soft p-4 text-sm leading-relaxed text-leaf-deep ring-1 ring-leaf-soft">
              If an account exists for <b>{email.trim()}</b>, a reset link is on its way. It can take a few minutes.
            </p>
          ) : (
            <form onSubmit={recover} noValidate className="space-y-4">
              <Input icon={Mail} aria-label="E-mail" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@company.com" disabled={!auth0} />
              {touched && !emailOk && <p role="alert" className="text-xs text-red-600">Enter a valid e-mail address.</p>}
              {error && <p role="alert" className="text-xs text-red-600">{error}</p>}
              {!auth0 && <p className="text-xs text-slate-500">Password recovery needs the identity provider and is off in this test environment.</p>}
              <button type="submit" disabled={busy || !auth0} className={primary}>{busy ? "Sending…" : "Send reset link"}</button>
            </form>
          )}
        </div>
      </div>
    );
  }

  if (view === "signup") {
    return (
      <div key="signup" className="form-swap">
        {back}
        <h1 className="text-[26px] font-semibold tracking-tight text-slate-900">Create your account</h1>
        <p className="mt-1.5 text-sm text-slate-500">Start measuring cost and carbon in minutes. Your organization is created for you.</p>
        <div className="mt-6 space-y-5">
          {socials("Sign up")}
          <Divider>or with e-mail</Divider>
          <form
            onSubmit={(e) => { e.preventDefault(); setTouched(true); if (auth0 && emailOk) window.location.assign(loginUrl({ email, returnTo, signup: true })); }}
            noValidate className="space-y-4"
          >
            <Input icon={User} aria-label="Full name" autoComplete="name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Full name" disabled={!auth0} />
            <Input icon={Mail} aria-label="Work e-mail" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@company.com" disabled={!auth0} aria-invalid={touched && !emailOk} />
            {touched && !emailOk && <p role="alert" className="text-xs text-red-600">Enter a valid e-mail address.</p>}
            <button type="submit" disabled={!auth0} className={primary}>Create account</button>
          </form>
          <p className="text-center text-xs leading-relaxed text-slate-500">You will choose your password on the identity provider's secure page.</p>
        </div>
      </div>
    );
  }

  return (
    <div key="signin" className="form-swap">
      <h1 className="text-[26px] font-semibold tracking-tight text-slate-900">Welcome back</h1>
      <p className="mt-1.5 text-sm text-slate-500">Sign in to your GreenOps and FinOps workspace.</p>
      <div className="mt-6 space-y-5">
        {socials("Continue")}
        <Divider>or</Divider>

        {mode === "dev" && devLogin ? (
          <form method="post" action="/api/dev/login" className="space-y-4">
            <input type="hidden" name="returnTo" value={returnTo} />
            <Input icon={User} name="username" id="username" aria-label="User name" autoComplete="username" required autoFocus placeholder="User name" />
            <div className="relative">
              <Input icon={Lock} name="password" id="password" aria-label="Password" type={show ? "text" : "password"} autoComplete="current-password" required placeholder="Password" className="pr-11" />
              <button type="button" onClick={() => setShow((v) => !v)} aria-label={show ? "Hide password" : "Show password"}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-slate-400 transition-colors hover:text-slate-700">
                {show ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
              </button>
            </div>
            {loginError && <p role="alert" className="text-xs text-red-600">{loginError === "rate" ? "Too many attempts. Wait a minute and try again." : "The user name or the password is not correct."}</p>}
            <button type="submit" className={primary}>Sign in</button>
            <p className="text-center text-xs text-slate-400">Test environment</p>
          </form>
        ) : mode === "dev" ? (
          <a href="/overview" className={cn(primary, "flex items-center justify-center")}>Continue as the development user</a>
        ) : (
          <form
            onSubmit={(e) => { e.preventDefault(); setTouched(true); if (emailOk) window.location.assign(loginUrl({ email, returnTo })); }}
            noValidate className="space-y-4"
          >
            <Input icon={Mail} id="email" aria-label="Work e-mail" type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@company.com" aria-invalid={touched && !emailOk} />
            {touched && !emailOk && <p role="alert" className="-mt-2 text-xs text-red-600">Enter a valid e-mail address.</p>}
            <button type="submit" className={primary}>Continue</button>
          </form>
        )}

        <div className="flex items-center justify-between text-sm">
          <button type="button" onClick={() => go("forgot")} className="font-medium text-leaf-deep transition-colors hover:text-leaf-deep hover:underline">Forgot your password?</button>
          <button type="button" onClick={() => go("signup")} className="font-medium text-slate-600 transition-colors hover:text-slate-900">Create an account →</button>
        </div>
        {auth0 && <p className="text-center text-xs leading-relaxed text-slate-400">Your password and, if turned on, your two-step code are entered on our identity provider's secure page.</p>}
      </div>
    </div>
  );
}
