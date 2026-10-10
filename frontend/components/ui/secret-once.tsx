"use client";

import { Check, Copy, KeyRound } from "lucide-react";
import { useState } from "react";

/** A secret the API shows exactly once (API key, invitation token, webhook signing secret). */
export function SecretOnce({ title, value, onDismiss }: { title: string; value: string; onDismiss: () => void }) {
  const [copied, setCopied] = useState(false);
  return (
    <div role="status" className="rounded-xl bg-amber-50 p-4 text-sm ring-1 ring-amber-200">
      <p className="flex items-center gap-2 font-semibold text-amber-900"><KeyRound className="size-4" />{title}</p>
      <p className="mt-1 text-xs text-amber-800">Copy it now. It is not stored in a readable form and cannot be shown again.</p>
      <div className="mt-2 flex items-center gap-2">
        <code className="min-w-0 flex-1 overflow-x-auto rounded-lg bg-white px-3 py-2 font-mono text-xs ring-1 ring-amber-200">{value}</code>
        <button type="button" aria-label="Copy" onClick={() => { void navigator.clipboard?.writeText(value).then(() => setCopied(true)); }}
          className="rounded-lg bg-white p-2 ring-1 ring-amber-200 hover:bg-amber-100">{copied ? <Check className="size-4" /> : <Copy className="size-4" />}</button>
      </div>
      <button type="button" onClick={onDismiss} className="mt-3 text-xs font-medium text-amber-900 underline">I have saved it</button>
    </div>
  );
}
