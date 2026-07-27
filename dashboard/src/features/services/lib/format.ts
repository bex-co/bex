// Render's root-directory affordance (w5/m48/t004): the "<rootDir>/" prefix
// shown before Build Command and Publish directory values, so both fields agree
// on the same trim/strip-trailing-slash rule. Empty when no root dir is set;
// callers append their own suffix (the Build Command prompt adds " $").
export function rootDirPrefix(rootDir: string | null | undefined): string {
  const dir = rootDir?.trim().replace(/\/+$/, "");
  return dir ? dir + "/" : "";
}

// Render's command-input prompt (w5/m51): the "<rootDir>/ $" prefix shown inside
// the Build / Pre-Deploy / Start Command inputs so a command reads as relative to
// the root directory. Falls back to a bare "$" prompt when no root dir is set.
export function commandPromptPrefix(rootDir: string | null | undefined): string {
  const prefix = rootDirPrefix(rootDir);
  return prefix ? `${prefix} $` : "$";
}

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
