import type { Translate } from "@/common/hooks/use-translations";

/** The API keys table's columns, shared by the panel and its frame. */
export function apiKeysColumns(t: Translate): string[] {
  return [
    t("apiKeys.colName"),
    t("apiKeys.colClientId"),
    t("apiKeys.colCreated"),
    t("apiKeys.colCreatedBy"),
    t("apiKeys.colLastUsed"),
  ];
}
