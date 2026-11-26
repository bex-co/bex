import { resourcePageHead } from "@/common/lib/document-head";

/**
 * Document title for one of a Key Value instance's pages (w3/m649): "Logs ・
 * sessions-cache ・ Key Value ・ bex Dashboard".
 */
export function keyValuePageHead(labelKey: string) {
  return resourcePageHead(
    "/keyvalue/$keyValueId",
    "keyvalue.resourceType",
    labelKey,
  );
}
