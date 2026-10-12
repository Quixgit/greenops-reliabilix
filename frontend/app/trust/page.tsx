import Link from "next/link";
import { Logo } from "@/components/layout/logo";
import { getLocale, getMessages } from "@/i18n/server";
import { translate } from "@/i18n/translate";

export const dynamic = "force-dynamic";

// The page is public (no sign-in): it describes how the platform protects data. Every claim is written in the message catalogs,
// next to the security model in docs/security/model.md; keep them in step.
const SECTIONS = [
  { id: "isolation", items: 3 },
  { id: "access", items: 3 },
  { id: "cloud", items: 2 },
  { id: "data", items: 3 },
  { id: "subprocessors", items: 4 },
  { id: "honest", items: 3 },
] as const;

export async function generateMetadata() {
  const locale = await getLocale();
  const { messages, fallback } = await getMessages(locale);
  return { title: translate(messages, fallback, locale, "trust.meta") };
}

export default async function TrustPage() {
  const locale = await getLocale();
  const { messages, fallback } = await getMessages(locale);
  const t = (key: string) => translate(messages, fallback, locale, key);
  return (
    <main className="mx-auto max-w-3xl px-6 py-12 text-slate-900">
      <Link href="/" className="inline-flex items-center gap-3">
        <Logo className="h-10 w-8" />
        <span className="text-lg font-bold tracking-tight">Reliabilix</span>
      </Link>
      <h1 className="mt-8 text-3xl font-bold tracking-tight">{t("trust.title")}</h1>
      <p className="mt-3 text-slate-600">{t("trust.lead")}</p>
      <div className="mt-10 space-y-8">
        {SECTIONS.map((s) => (
          <section key={s.id} className={s.id === "honest" ? "rounded-2xl bg-amber-50/70 p-5" : ""}>
            <h2 className="text-lg font-semibold">{t(`trust.${s.id}.title`)}</h2>
            <ul className="mt-3 list-disc space-y-2 pl-5 text-sm leading-relaxed text-slate-700">
              {Array.from({ length: s.items }, (_, n) => <li key={n}>{t(`trust.${s.id}.${n}`)}</li>)}
            </ul>
          </section>
        ))}
      </div>
    </main>
  );
}
