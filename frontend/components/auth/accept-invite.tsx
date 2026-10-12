"use client";

import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { tenantRef } from "@/lib/api/schemas";
import { useT } from "@/i18n/provider";

/** One click accepts: a button, not an effect, so a link previewer or a double render can never use the one-time token. */
export function AcceptInvite({ token }: { token: string }) {
  const t = useT();
  const router = useRouter();
  const m = useMutation({
    mutationFn: () => api("invitations/accept", tenantRef, undefined, { method: "POST", body: JSON.stringify({ token }) }),
    onSuccess: () => router.replace("/overview"),
  });
  return (
    <div className="space-y-3">
      <button onClick={() => m.mutate()} disabled={m.isPending} className="brand-gradient w-full rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">
        {m.isPending ? t("invite.joining") : t("invite.join")}
      </button>
      {m.isError && <p role="alert" className="text-sm text-bad">{t("invite.failed")}</p>}
    </div>
  );
}
