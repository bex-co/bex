import type { Translate } from "@/common/hooks/use-translations";

/** The sessions table's columns, shared by the panel and its frame. */
export function activeSessionsColumns(t: Translate): string[] {
  return [
    t("activeSessions.colDevice"),
    t("activeSessions.colLocation"),
    t("activeSessions.colSignedIn"),
  ];
}
