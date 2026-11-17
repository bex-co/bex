import type { Translate } from "@/common/hooks/use-translations";

/** The connected agents table's columns, shared by the panel and its frame. */
export function connectedAgentsColumns(t: Translate): string[] {
  return [
    t("connectedAgents.colClient"),
    t("connectedAgents.colScopes"),
    t("connectedAgents.colGranted"),
  ];
}
