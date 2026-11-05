import { canonicalLinkId } from "@/common/lib/link-id";

/** Keep saved dashboard links compatible with the API's public exs- Blueprint
 * IDs (w2/m269). bex-api accepts both spellings; rewriting the historical
 * blp- shape keeps every query and cache entry on the one ID the API returns. */
export function canonicalBlueprintLinkId(value: string): string {
  return canonicalLinkId(value, "blp", "exs");
}
