// Compact relative age in Render's dashboard style ("2mo", "5d", "3h", "4m",
// "now"). Render's services list shows this in its "Updated" column; bex only
// tracks a creation timestamp today (a true last-deploy time is a known gap), so
// the UI labels it "Created" and feeds `createdAt` here. `now` is injectable so
// the output is deterministic under test.
export function formatRelativeAge(
  iso: string | null | undefined,
  now: number = Date.now(),
): string {
  if (!iso) return "—";
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return "—";

  const secs = Math.max(0, Math.floor((now - then) / 1000));
  const mins = Math.floor(secs / 60);
  const hours = Math.floor(mins / 60);
  const days = Math.floor(hours / 24);
  const months = Math.floor(days / 30);
  const years = Math.floor(days / 365);

  if (secs < 60) return "now";
  if (mins < 60) return `${mins}m`;
  if (hours < 24) return `${hours}h`;
  if (days < 30) return `${days}d`;
  if (months < 12) return `${months}mo`;
  return `${years}y`;
}
