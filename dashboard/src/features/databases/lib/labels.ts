import type { en } from "@/i18n";
import type { DatabaseStatusKey } from "@/features/databases/types";

/**
 * Maps a resolved database status key to its i18n badge label. Shared by the
 * list rows and the detail header so the same status renders the same word
 * everywhere (mirrors services' STATUS_LABEL).
 */
export const STATUS_LABEL: Record<DatabaseStatusKey, keyof typeof en> = {
  available: "databases.statusAvailable",
  creating: "databases.statusCreating",
  unavailable: "databases.statusUnavailable",
  unknown: "databases.statusUnknown",
};
