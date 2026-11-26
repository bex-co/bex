import type {
  DatabaseDetailView,
  DatabaseInstanceTypeView,
} from "@/features/databases/types";

/** Render sells Postgres storage in 5 GB steps. */
export const STORAGE_STEP_GB = 5;

/**
 * A plan whose included storage is its ceiling (Free: 1 GB) has a fixed disk:
 * bex-api refuses any size change on it, as Render does.
 */
export function planStorageFixed(
  plan: Pick<DatabaseInstanceTypeView, "storageGB" | "maxStorageGB">,
): boolean {
  return plan.maxStorageGB != null && plan.storageGB === plan.maxStorageGB;
}

/** The database's allocated storage, or its plan's included size if unread. */
export function databaseStorageGB(
  database: Pick<DatabaseDetailView, "diskSizeGB">,
  plan: Pick<DatabaseInstanceTypeView, "storageGB">,
): number {
  return database.diskSizeGB ?? plan.storageGB;
}

export type StorageGrowProblem = "step" | "notGreater" | "tooLarge";

/**
 * Why a requested manual increase would be refused: Render takes "any multiple
 * of 5 GB greater than the current storage capacity", up to the plan maximum.
 */
export function storageGrowProblem(
  value: string,
  current: number,
  max: number | null,
): StorageGrowProblem | null {
  const trimmed = value.trim();
  const gb = Number(trimmed);
  if (!/^\d+$/.test(trimmed) || gb % STORAGE_STEP_GB !== 0) return "step";
  if (gb <= current) return "notGreater";
  if (max != null && gb > max) return "tooLarge";
  return null;
}

/** Render's GiB in its almost-full rule (`1073741824`). */
const BYTES_PER_GIB = 1024 ** 3;
/** Render warns once more than this percent of the storage is used… */
const ALMOST_FULL_USED_PERCENT = 80;
/** …and less than this many GiB are left. */
const ALMOST_FULL_FREE_GIB = 5;

/** The usage fields Render's rule reads (GraphQL `Database`, w3/m656). */
export interface DatabaseStorageUsage {
  storageUsedPercent: number | null;
  /** The logical size in bytes, a decimal string. */
  storageTotal: string | null;
}

/**
 * Render's almost-full rule (w3/m656, docs/render-artifacts/postgres-dashboard.md
 * § Almost-full warning): more than 80% of the storage used and less than
 * 5 GiB left, so a large database at 95% with plenty left does not warn. A
 * missing or zero number never warns, as on Render.
 */
export function storageAlmostFull(
  usage: DatabaseStorageUsage | null | undefined,
): boolean {
  const total = Number(usage?.storageTotal);
  const percent = Number(usage?.storageUsedPercent);
  if (!percent || !total) return false;
  const freeGiB = (1 - percent / 100) * (total / BYTES_PER_GIB);
  return percent > ALMOST_FULL_USED_PERCENT && freeGiB < ALMOST_FULL_FREE_GIB;
}

/**
 * What the almost-full warning offers, Render's three endings: grow the
 * storage on the Plan page; a plan whose storage is fixed upgrades the plan
 * instead; storage already at its plan's maximum, which no plan grows
 * further, asks support. An unread plan offers the storage control.
 */
type AlmostFullAction = "increaseStorage" | "upgradePlan" | "support";

export function almostFullAction(
  database: Pick<DatabaseDetailView, "diskSizeGB">,
  plan:
    Pick<DatabaseInstanceTypeView, "storageGB" | "maxStorageGB"> | undefined,
): AlmostFullAction {
  if (!plan) return "increaseStorage";
  if (planStorageFixed(plan)) return "upgradePlan";
  const max = plan.maxStorageGB;
  return max != null && databaseStorageGB(database, plan) >= max
    ? "support"
    : "increaseStorage";
}
