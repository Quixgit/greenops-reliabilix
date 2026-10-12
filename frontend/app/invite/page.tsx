import { redirect } from "next/navigation";
import { AcceptInvite } from "@/components/auth/accept-invite";
import { Logo } from "@/components/layout/logo";
import { getLocale, getMessages } from "@/i18n/server";
import { translate } from "@/i18n/translate";
import { getUser } from "@/lib/auth/session";

export const dynamic = "force-dynamic";

// The link of an invitation e-mail: /invite?token=…. Signing in comes first (and brings the person back here); the token is used by a click.
export default async function InvitePage({ searchParams }: { searchParams: Promise<{ token?: string }> }) {
  const { token } = await searchParams;
  const locale = await getLocale();
  const { messages, fallback } = await getMessages(locale);
  const t = (key: string) => translate(messages, fallback, locale, key);
  if (typeof token === "string" && token && !(await getUser())) redirect(`/login?returnTo=${encodeURIComponent(`/invite?token=${encodeURIComponent(token)}`)}`);
  return (
    <main className="flex min-h-screen items-center justify-center p-6">
      <div className="glass w-full max-w-md space-y-5 p-8">
        <Logo className="size-12" />
        <div>
          <h1 className="text-xl font-bold">{t("invite.title")}</h1>
          <p className="mt-1 text-sm text-muted">{t(token ? "invite.lead" : "invite.missing")}</p>
        </div>
        {token && <AcceptInvite token={token} />}
      </div>
    </main>
  );
}
