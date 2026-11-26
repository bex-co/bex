import { resourcePageHead } from "@/common/lib/document-head";

/** A sandbox group page's title: the page, the group, "Sandbox group". */
export function sandboxGroupPageHead(labelKey: string) {
  return resourcePageHead(
    "/sandbox-groups_/$sandboxGroupId",
    "sandboxGroups.resourceType",
    labelKey,
  );
}
