import type { en } from "@/i18n";
import { deriveStatus } from "@/features/databases/lib/status";
import type { DatabaseStatusKey } from "@/features/databases/types";

/**
 * Maps a resolved database status key to its i18n badge label. Shared by the
 * list rows and the detail header so the same status renders the same word
 * everywhere (mirrors services' STATUS_LABEL).
 */
export const STATUS_LABEL: Record<DatabaseStatusKey, keyof typeof en> = {
  available: "databases.statusAvailable",
  creating: "databases.statusCreating",
  upgrading: "databases.statusUpgrading",
  unavailable: "databases.statusUnavailable",
  suspended: "databases.statusSuspended",
  unknown: "databases.statusUnknown",
};

/**
 * The i18n label for a database's displayed status — one composition shared by
 * the badge and the detail page's Details row, so a suspended instance can
 * never read "Suspended" in one and the raw "available" in the other (w1/m159).
 */
export function statusLabel(d: {
  status: string;
  suspended: string;
}): keyof typeof en {
  return STATUS_LABEL[deriveStatus(d).key];
}
