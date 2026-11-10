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
