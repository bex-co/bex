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
