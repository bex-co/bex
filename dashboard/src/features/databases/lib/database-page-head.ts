import { resourcePageHead } from "@/common/lib/document-head";

/**
 * Document title for one of a Postgres database's pages (w3/m641): "Recovery
 * ・ orders ・ Database ・ bex Dashboard".
 */
export function databasePageHead(labelKey: string) {
  return resourcePageHead(
    "/databases/$databaseId",
    "databases.resourceType",
    labelKey,
  );
}
