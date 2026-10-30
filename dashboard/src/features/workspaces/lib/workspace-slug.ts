import { uncappedSlug } from "@/common/lib/utils/resource-slug";

/** The longest slug isValidDnsLabel accepts. */
const MAX_WORKSPACE_SLUG_LENGTH = 30;

/**
 * The workspace slug a freeform name suggests (w3/m246): the same mapping
 * bex-api gives a service name (uncappedSlug), cut to the slug length limit.
 */
export function suggestWorkspaceSlug(name: string): string {
  return uncappedSlug(name)
    .slice(0, MAX_WORKSPACE_SLUG_LENGTH)
    .replace(/-+$/, "");
}
