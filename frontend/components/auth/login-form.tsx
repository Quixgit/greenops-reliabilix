"use client";

import { ArrowLeft, Eye, EyeOff, Mail } from "lucide-react";
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

const input = "block h-11 w-full rounded-lg border border-slate-200 bg-white/90 px-3.5 text-sm outline-none transition placeholder:text-slate-400 focus:border-emerald-500 focus:ring-2 focus:ring-emerald-500/25 disabled:bg-slate-50";
const primary = "h-11 w-full rounded-lg bg-gradient-to-b from-emerald-500 to-emerald-600 text-sm font-semibold text-white shadow-sm transition hover:from-emerald-500 hover:to-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500/40 disabled:opacity-60";
const social = "flex h-11 flex-1 items-center justify-center gap-2 rounded-lg border border-slate-200 bg-white text-sm font-medium text-slate-800 transition hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-emerald-500/25";

/**
 * One calm form. In the test environment a user name and a password; with the identity provider an e-mail, with Google
 * and GitHub next to it. Passwords and second factors are only ever typed on the provider's own page.
 */
export function LoginForm({ mode, returnTo, devLogin = false, error: loginError }: { mode: "auth0" | "dev"; returnTo: string; devLogin?: boolean; error?: string }) {
  const [view, setView] = useState<"signin" | "forgot">("signin");
  const [email, setEmail] = useState("");
  const [show, setShow] = useState(false);
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
        <button type="button" onClick={() => { setView("signin"); setSent(false); setError(""); }} className="inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-900">
          <ArrowLeft className="size-4" />Back
        </button>
        <div>
          <h1 className="text-[22px] font-semibold tracking-tight text-slate-900">Reset your password</h1>
          <p className="mt-1.5 text-sm text-slate-500">We will send a link to choose a new one.</p>
        </div>
        {sent ? (
          <p role="status" className="rounded-lg bg-emerald-50 p-3.5 text-sm text-emerald-900 ring-1 ring-emerald-200">
            If an account exists for <b>{email.trim()}</b>, a reset link is on its way. It can take a few minutes.
          </p>
        ) : (
          <form onSubmit={recover} noValidate className="space-y-3">
            <input aria-label="E-mail" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} className={input} placeholder="you@company.com" disabled={!auth0} />
            {touched && !emailOk && <p role="alert" className="text-xs text-red-600">Enter a valid e-mail address.</p>}
            {error && <p role="alert" className="text-xs text-red-600">{error}</p>}
            {!auth0 && <p className="text-xs text-slate-500">Password recovery needs the identity provider and is off in this test environment.</p>}
            <button type="submit" disabled={busy || !auth0} className={primary}>{busy ? "Sending…" : "Send reset link"}</button>
          </form>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-[26px] font-semibold tracking-tight text-slate-900">Sign in</h1>
        <p className="mt-1 text-sm text-slate-500">to Reliabilix GreenOps</p>
      </div>

      {mode === "dev" && devLogin ? (
        <form method="post" action="/api/dev/login" className="space-y-3.5">
          <input type="hidden" name="returnTo" value={returnTo} />
          <div className="space-y-1.5">
            <label htmlFor="username" className="text-xs font-medium text-slate-600">User name</label>
            <input id="username" name="username" autoComplete="username" required autoFocus className={input} />
          </div>
          <div className="space-y-1.5">
            <label htmlFor="password" className="text-xs font-medium text-slate-600">Password</label>
            <div className="relative">
              <input id="password" name="password" type={show ? "text" : "password"} autoComplete="current-password" required className={cn(input, "pr-11")} />
              <button type="button" onClick={() => setShow((v) => !v)} aria-label={show ? "Hide password" : "Show password"}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-slate-400 hover:text-slate-700">
                {show ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
              </button>
            </div>
          </div>
          {loginError && <p role="alert" className="text-xs text-red-600">{loginError === "rate" ? "Too many attempts. Wait a minute and try again." : "The user name or the password is not correct."}</p>}
          <button type="submit" className={primary}>Sign in</button>
          <p className="pt-1 text-center text-xs text-slate-500">Test environment</p>
        </form>
      ) : mode === "dev" ? (
        <a href="/overview" className={cn(primary, "flex items-center justify-center")}>Continue as the development user</a>
      ) : (
        <>
          <div className="flex gap-3">
            <a href={loginUrl({ connection: "google-oauth2", returnTo })} className={social}><GoogleMark />Google</a>
            <a href={loginUrl({ connection: "github", returnTo })} className={social}><GitHubMark />GitHub</a>
          </div>
          <div className="flex items-center gap-3 text-xs text-slate-400"><span className="h-px flex-1 bg-slate-200" />or<span className="h-px flex-1 bg-slate-200" /></div>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              setTouched(true);
              if (emailOk) window.location.assign(loginUrl({ email, returnTo }));
            }}
            noValidate className="space-y-3.5"
          >
            <div className="space-y-1.5">
              <label htmlFor="email" className="text-xs font-medium text-slate-600">Work e-mail</label>
              <div className="relative">
                <Mail className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-slate-400" />
                <input id="email" type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@company.com" className={cn(input, "pl-10")} aria-invalid={touched && !emailOk} />
              </div>
              {touched && !emailOk && <p role="alert" className="text-xs text-red-600">Enter a valid e-mail address.</p>}
            </div>
            <button type="submit" className={primary}>Continue</button>
            <div className="text-center">
              <button type="button" onClick={() => setView("forgot")} className="text-sm font-medium text-emerald-700 hover:underline">Forgot your password?</button>
            </div>
          </form>
          <p className="text-center text-xs leading-relaxed text-slate-500">
            Your password and, if turned on, your two-step code are entered on our identity provider's secure page.
          </p>
        </>
      )}
    </div>
  );
}
