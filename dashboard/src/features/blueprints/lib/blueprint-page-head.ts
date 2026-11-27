import { resourcePageHead } from "@/common/lib/document-head";

/**
 * Document title for one of a Blueprint's pages (w3/m677): "Syncs ・ platform
 * ・ Blueprint ・ bex Dashboard", the page named first as on bex's other
 * resource pages.
 */
export function blueprintPageHead(labelKey: string) {
  return resourcePageHead(
    "/blueprints/$blueprintId",
    "blueprints.resourceType",
    labelKey,
  );
}
