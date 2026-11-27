import { createLocalSetting } from "@/features/logs/lib/local-setting";
import type {
  DatabaseDetailView,
  DatabaseUpgradeRunView,
} from "@/features/databases/types";

// The guided version-upgrade page's rules (w3/m676), as Render's
// VersionUpgrade page applies them (docs/render-artifacts/
// postgres-version-upgrade.md § The guided page).

/** Render's upgrade run states (bex-api `lastUpgradeRun.state`). */
export const UPGRADE_IN_PROGRESS = "IN_PROGRESS";
export const UPGRADE_SUCCEEDED = "SUCCEEDED";
export const UPGRADE_FAILED = "FAILED";

/** bex's docs on major-version upgrades, where Render links its guide. */
export const VERSION_UPGRADE_DOCS_HREF =
  "https://bex.co/docs/platform/postgres#upgrade-or-remove-a-database";

/** PostgreSQL's release notes for a major, Render's "Read release notes". */
export function releaseNotesHref(version: string): string {
  return `https://www.postgresql.org/docs/${version}/release-${version}.html`;
}

/**
 * Whether the database has the page at all: Render offers upgrades on a
 * paid primary only (its `t` rule: plan not free, role primary), and bex has
 * no replica pages, so a paid plan is the whole rule.
 */
export function versionUpgradeEligible(
  database: Pick<DatabaseDetailView, "plan">,
): boolean {
  return database.plan !== "free";
}

/** Whether the database runs a major older than the upgrade target. */
export function upgradeAvailable(
  database: Pick<
    DatabaseDetailView,
    "plan" | "version" | "versionUpgradeAvailable"
  >,
): boolean {
  const target = Number(database.versionUpgradeAvailable);
  const current = Number(database.version);
  return (
    versionUpgradeEligible(database) &&
    Number.isFinite(target) &&
    Number.isFinite(current) &&
    current > 0 &&
    current < target
  );
}

export function upgradeRunning(run: DatabaseUpgradeRunView | null | undefined) {
  return run?.state === UPGRADE_IN_PROGRESS;
}

export function upgradeFinished(
  run: DatabaseUpgradeRunView | null | undefined,
) {
  return run?.state === UPGRADE_SUCCEEDED || run?.state === UPGRADE_FAILED;
}

/** A failed run nobody dismissed: Render's "Upgrade failed." banner. */
export function unacknowledgedFailure(
  run: DatabaseUpgradeRunView | null | undefined,
): run is DatabaseUpgradeRunView {
  return run?.state === UPGRADE_FAILED && !run.acknowledged;
}

// A clone made for a test upgrade is named `<source>-upgrade-clone-<4>`, as
// Render names it, and only such a clone shows the clone banner and skips
// Render's PITR wait before its own upgrade.
const UPGRADE_CLONE_NAME = /-upgrade-clone-[a-z0-9]{4}$/i;

export function isUpgradeClone(name: string): boolean {
  return UPGRADE_CLONE_NAME.test(name);
}

const CLONE_SUFFIX_ALPHABET = "abcdefghijklmnopqrstuvwxyz0123456789";

/** A fresh clone name for `source`: Render's `${name}-upgrade-clone-${4}`. */
export function upgradeCloneName(
  source: string,
  random: () => number = Math.random,
): string {
  let suffix = "";
  for (let i = 0; i < 4; i++) {
    suffix +=
      CLONE_SUFFIX_ALPHABET[
        Math.floor(random() * CLONE_SUFFIX_ALPHABET.length)
      ];
  }
  return `${source}-upgrade-clone-${suffix}`;
}

/** One side of a clone pairing: a database's id and name. */
export interface UpgradeCloneRef {
  id: string;
  name: string;
}

/**
 * Which clone belongs to which source, kept in this browser as Render keeps
 * it (`postgres-upgrade-clone-info`, keyed both ways): the clone's banner
 * names its source, and the source's page links its clone again.
 */
type CloneInfo = Record<string, UpgradeCloneRef>;

const cloneInfoSetting = createLocalSetting<string>(
  "postgres-upgrade-clone-info",
  "{}",
  (raw) => raw ?? "{}",
);

function parseCloneInfo(raw: string): CloneInfo {
  try {
    const parsed: unknown = JSON.parse(raw);
    return parsed && typeof parsed === "object" ? (parsed as CloneInfo) : {};
  } catch {
    return {};
  }
}

function isRef(value: unknown): value is UpgradeCloneRef {
  return (
    typeof value === "object" &&
    value !== null &&
    typeof (value as UpgradeCloneRef).id === "string" &&
    typeof (value as UpgradeCloneRef).name === "string"
  );
}

/** The database paired with `id` in this browser, if any. */
export function useUpgradeClonePair(id: string): UpgradeCloneRef | null {
  const pair = parseCloneInfo(cloneInfoSetting.useValue())[id];
  return isRef(pair) ? pair : null;
}

/** Records a new clone of `source`, both ways. */
export function rememberUpgradeClone(
  clone: UpgradeCloneRef,
  source: UpgradeCloneRef,
) {
  const info = parseCloneInfo(cloneInfoSetting.read());
  cloneInfoSetting.write(
    JSON.stringify({ ...info, [clone.id]: source, [source.id]: clone }),
  );
}

// Render's Info banner that announces a newer major can be dismissed, per
// database, in this browser.
const dismissedAvailableSetting = createLocalSetting<string>(
  "postgres-upgrade-available-dismissed",
  "[]",
  (raw) => raw ?? "[]",
);

function parseIds(raw: string): string[] {
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed)
      ? parsed.filter((v): v is string => typeof v === "string")
      : [];
  } catch {
    return [];
  }
}

export function useUpgradeAvailableDismissed(id: string): boolean {
  return parseIds(dismissedAvailableSetting.useValue()).includes(id);
}

export function dismissUpgradeAvailable(id: string) {
  const current = parseIds(dismissedAvailableSetting.read());
  if (current.includes(id)) return;
  dismissedAvailableSetting.write(JSON.stringify([...current, id]));
}
