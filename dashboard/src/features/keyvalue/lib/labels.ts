import type { en } from "@/i18n";
import type { KeyValueStatusKey } from "@/features/keyvalue/types";

/**
 * Maps a resolved Key Value status key to its i18n badge label. Shared by the
 * list rows and the detail header so the same status renders the same word
 * everywhere (mirrors databases' STATUS_LABEL).
 */
export const STATUS_LABEL: Record<KeyValueStatusKey, keyof typeof en> = {
  available: "keyvalue.statusAvailable",
  creating: "keyvalue.statusCreating",
  unavailable: "keyvalue.statusUnavailable",
  unknown: "keyvalue.statusUnknown",
};
