import type { Translate } from "@/common/hooks/use-translations";

/** Where the Permissions column sits in `apiKeysColumns`. */
export const API_KEYS_PERMISSIONS_COLUMN = 1;

/** The API keys table's columns, shared by the panel and its frame. */
export function apiKeysColumns(t: Translate): string[] {
  return [
    t("apiKeys.colName"),
    t("apiKeys.colPermissions"),
    t("apiKeys.colClientId"),
    t("apiKeys.colCreated"),
    t("apiKeys.colCreatedBy"),
    t("apiKeys.colLastUsed"),
  ];
}
