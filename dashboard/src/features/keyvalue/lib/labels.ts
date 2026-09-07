import type { en } from "@/i18n";
import type { KeyValueStatusKey } from "@/features/keyvalue/types";

/**
 * Maps a resolved Key Value status key to its i18n badge label. Shared by the
 * list rows and the detail header so the same status renders the same word
 * everywhere (mirrors databases' STATUS_LABEL).
 */
export const STATUS_LABEL: Record<KeyValueStatusKey, keyof typeof en> = {
  available: "keyvalue.statusAvailable",
  creating: "keyvalue.statusCreating",
  unavailable: "keyvalue.statusUnavailable",
  suspended: "keyvalue.statusSuspended",
  unknown: "keyvalue.statusUnknown",
};

// Maxmemory (eviction) policies bex offers, matching the KeyValue CRD's enum
// (lego/types/v1alpha1/keyvalue_types.go) and Render's Key Value form
// (docs/render-artifacts/key-value.md). `allkeys-lru` leads — Render's
// cache-oriented default. Shared by the create wizard and the detail-page
// editor (w7/007) so both offer the same ladder from one source.
export const MAXMEMORY_POLICIES = [
  "allkeys-lru",
  "allkeys-lfu",
  "volatile-lru",
  "volatile-lfu",
  "allkeys-random",
  "volatile-random",
  "volatile-ttl",
  "noeviction",
] as const;

/** The recommended-for-caches default; also the create wizard's initial value. */
export const RECOMMENDED_MAXMEMORY_POLICY: (typeof MAXMEMORY_POLICIES)[number] =
  "allkeys-lru";

/**
 * bex-api reads the eviction policy back with UNDERSCORES (`allkeys_lfu`,
 * keyvalue/service.go), while the UI option vocabulary — and the CRD/save path —
 * spells it with HYPHENS (`allkeys-lfu`). Map the read onto the UI form so the
 * detail-page selector shows the saved policy instead of a blank when the two
 * spellings disagree (w4/046). `noeviction` (no separator) is unchanged either
 * way. Empty (a pending/absent read) and any value that doesn't resolve to a
 * known policy pass through untouched — never fabricate a policy the store did
 * not report. Save is unaffected: it already sends the hyphen UI value, which
 * the backend accepts. Wire spelling on REST/GraphQL/MCP stays underscored.
 */
export function maxmemoryPolicyToUi(apiValue: string): string {
  const hyphenated = apiValue.replace(/_/g, "-");
  return (MAXMEMORY_POLICIES as readonly string[]).includes(hyphenated)
    ? hyphenated
    : apiValue;
}
