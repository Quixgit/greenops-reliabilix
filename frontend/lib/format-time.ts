/** "2026-10-09T12:00:00Z" -> "9 Oct 12:00" (local time, short); the raw value stays in the title attribute where shown. */
export function shortTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString(undefined, { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
}
