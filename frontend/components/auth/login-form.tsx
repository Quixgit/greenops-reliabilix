"use client";

import { ArrowLeft, Mail } from "lucide-react";
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

const field = "block w-full rounded-xl border border-line bg-white px-3.5 py-2.5 text-sm outline-none transition focus:border-primary focus:ring-2 focus:ring-primary/20";
const social = "flex w-full items-center justify-center gap-2.5 rounded-xl border border-line bg-white px-4 py-2.5 text-sm font-semibold text-ink transition hover:bg-slate-50";

/**
 * Sign-in options. Passwords and second factors are only ever typed on the identity provider's own page: this form
 * hands over the e-mail (and the chosen provider) and the provider asks for the rest, including the MFA code.
 */
export function LoginForm({ mode, returnTo, devLogin = false, error: loginError }: { mode: "auth0" | "dev"; returnTo: string; devLogin?: boolean; error?: string }) {
  const [view, setView] = useState<"signin" | "forgot">("signin");
  const [email, setEmail] = useState("");
  const [touched, setTouched] = useState(false);
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const auth0 = mode === "auth0";
  const emailOk = validEmail(email);

  async function recover(e: React.FormEvent) {
    e.preventDefault();
    setTouched(true);
    if (!emailOk) return;
    setBusy(true);
    setError("");
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

  if (view === "forgot") {
    return (
      <div className="space-y-5">
        <button type="button" onClick={() => { setView("signin"); setSent(false); setError(""); }} className="inline-flex items-center gap-1.5 text-sm text-muted hover:text-ink">
          <ArrowLeft className="size-4" />Back to sign in
        </button>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Reset your password</h1>
          <p className="mt-1.5 text-sm text-muted">Enter the e-mail address of your account and we will send you a link to choose a new password.</p>
        </div>
        {sent ? (
          <p role="status" className="rounded-xl bg-emerald-50 p-3.5 text-sm text-emerald-800 ring-1 ring-emerald-200">
            If an account exists for <b>{email.trim()}</b>, a reset link is on its way. It can take a few minutes; check your spam folder too.
          </p>
        ) : (
          <form onSubmit={recover} noValidate className="space-y-3">
            <label className="block text-sm font-medium" htmlFor="rec-email">Work e-mail</label>
            <input id="rec-email" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} className={field} placeholder="you@company.com" disabled={!auth0} />
            {touched && !emailOk && <p role="alert" className="text-xs text-bad">Enter a valid e-mail address.</p>}
            {error && <p role="alert" className="text-xs text-bad">{error}</p>}
            {!auth0 && <p className="text-xs text-muted">Password recovery needs the production identity provider and is switched off in this development environment.</p>}
            <button type="submit" disabled={busy || !auth0} className="brand-gradient w-full rounded-xl px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:opacity-95 disabled:opacity-60">
              {busy ? "Sending…" : "Send reset link"}
            </button>
          </form>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Welcome back</h1>
        <p className="mt-1.5 text-sm text-muted">Sign in to see your cloud cost and carbon footprint.</p>
      </div>

      {mode === "dev" && devLogin && (
        <form method="post" action="/api/dev/login" className="space-y-3 rounded-xl bg-violet-50 p-3.5 text-sm ring-1 ring-violet-200">
          <p className="text-violet-900"><b>Test environment.</b> Sign in with the access details you were given. Google, GitHub and password recovery are available once the identity provider is connected.</p>
          <input type="hidden" name="returnTo" value={returnTo} />
          <label className="block"><span className="text-xs font-medium text-violet-900">User name</span>
            <input name="username" autoComplete="username" required className={field} /></label>
          <label className="block"><span className="text-xs font-medium text-violet-900">Password</span>
            <input name="password" type="password" autoComplete="current-password" required className={field} /></label>
          {loginError && <p role="alert" className="text-xs text-bad">{loginError === "rate" ? "Too many attempts. Wait a minute and try again." : "The user name or the password is not correct."}</p>}
          <button type="submit" className="brand-gradient w-full rounded-xl px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:opacity-95">Sign in</button>
        </form>
      )}

      {mode === "dev" && !devLogin && (
        <div className="space-y-3 rounded-xl bg-violet-50 p-3.5 text-sm ring-1 ring-violet-200">
          <p className="text-violet-900"><b>Development environment.</b> Sign-in with Google, GitHub or a password is available once the identity provider is configured.</p>
          <a href="/overview" className="brand-gradient block rounded-lg px-4 py-2 text-center font-semibold text-white">Continue as the development user</a>
        </div>
      )}

      <div className="space-y-2.5">
        <a href={auth0 ? loginUrl({ connection: "google-oauth2", returnTo }) : undefined} aria-disabled={!auth0}
          className={cn(social, !auth0 && "pointer-events-none opacity-50")}><GoogleMark />Continue with Google</a>
        <a href={auth0 ? loginUrl({ connection: "github", returnTo }) : undefined} aria-disabled={!auth0}
          className={cn(social, !auth0 && "pointer-events-none opacity-50")}><GitHubMark />Continue with GitHub</a>
      </div>

      <div className="flex items-center gap-3 text-xs text-muted"><span className="h-px flex-1 bg-line" />or with your e-mail<span className="h-px flex-1 bg-line" /></div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          setTouched(true);
          if (emailOk && auth0) window.location.assign(loginUrl({ email, returnTo }));
        }}
        noValidate className="space-y-3"
      >
        <label className="block text-sm font-medium" htmlFor="email">Work e-mail</label>
        <div className="relative">
          <Mail className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-400" />
          <input id="email" type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@company.com"
            className={cn(field, "pl-9")} disabled={!auth0} aria-invalid={touched && !emailOk} />
        </div>
        {touched && !emailOk && <p role="alert" className="text-xs text-bad">Enter a valid e-mail address.</p>}
        <button type="submit" disabled={!auth0} className="brand-gradient w-full rounded-xl px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:opacity-95 disabled:opacity-60">
          Continue
        </button>
        <div className="flex items-center justify-between text-sm">
          <button type="button" onClick={() => setView("forgot")} className="font-medium text-primary hover:underline">Forgot your password?</button>
        </div>
      </form>

      <p className="rounded-xl bg-slate-50 p-3 text-xs leading-relaxed text-muted ring-1 ring-line">
        You enter your password — and, if two-step verification is turned on for your account, the code from your authenticator
        app — on the secure sign-in page of our identity provider. Reliabilix never sees your password.
      </p>
    </div>
  );
}
