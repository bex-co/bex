import type { Translate } from "@/common/hooks/use-translations";

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
export function commandPromptPrefix(
  rootDir: string | null | undefined,
): string {
  const prefix = rootDirPrefix(rootDir);
  return prefix ? `${prefix} $` : "$";
}

// Compact relative age in Render's dashboard style ("2mo", "5d", "3h", "4m",
// "now"; 2个月 / 5天 / 3小时 / 4分钟 / 刚刚 in Chinese). Render's services list
// shows this in its "Updated" column; bex only tracks a creation timestamp today
// (a true last-deploy time is a known gap), so the UI labels it "Created" and
// feeds `createdAt` here. `t` is the caller's active translator (never an
// ambient i18next singleton — SSR owns a request-scoped instance), and `now` is
// injectable so the output is deterministic under test.
export function formatRelativeAge(
  iso: string | null | undefined,
  t: Translate,
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

  if (secs < 60) return t("common.relativeAgeNow");
  if (mins < 60) return t("common.relativeAgeMinutes", { n: mins });
  if (hours < 24) return t("common.relativeAgeHours", { n: hours });
  if (days < 30) return t("common.relativeAgeDays", { n: days });
  if (months < 12) return t("common.relativeAgeMonths", { n: months });
  return t("common.relativeAgeYears", { n: years });
}

// Compact relative time until a FUTURE instant ("in 5m", "in 3h", "in 2d";
// 5分钟后 / 3小时后 / 2天后 in Chinese), the mirror of formatRelativeAge — used
// for a cron_job's computed next run. A time already in the past collapses to
// "now" (the scheduler is about to fire).
export function formatRelativeUntil(
  iso: string | null | undefined,
  t: Translate,
  now: number = Date.now(),
): string {
  if (!iso) return "—";
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return "—";

  const secs = Math.max(0, Math.floor((then - now) / 1000));
  const mins = Math.floor(secs / 60);
  const hours = Math.floor(mins / 60);
  const days = Math.floor(hours / 24);

  if (secs < 60) return t("common.relativeUntilNow");
  if (mins < 60) return t("common.relativeUntilMinutes", { n: mins });
  if (hours < 24) return t("common.relativeUntilHours", { n: hours });
  return t("common.relativeUntilDays", { n: days });
}
