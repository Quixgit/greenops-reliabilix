"use client";

export const input = "w-full rounded-xl bg-white/80 px-3 py-2 text-sm ring-1 ring-line outline-none focus:ring-2 focus:ring-leaf/40";
export function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) {
  return (
    <label className="block text-sm"><span className="mb-1 block font-medium text-slate-600">{label}</span>{children}
      {error && <span role="alert" className="mt-1 block text-xs text-bad">{error}</span>}</label>
  );
}
