import type { KeyValueInstanceTypeView } from "@/features/keyvalue/types";

/** Bytes per binary k8s quantity suffix — the spellings lego/types/tiers uses. */
const BINARY_UNIT_BYTES: Record<string, number> = {
  Ki: 1024,
  Mi: 1024 ** 2,
  Gi: 1024 ** 3,
  Ti: 1024 ** 4,
};

/**
 * A tier's memory as bytes, or null for a spelling this parser does not know
 * (bex-api sends the catalog's k8s quantities, e.g. "128Mi", "1Gi").
 */
export function tierMemoryBytes(memory: string): number | null {
  const match = /^(\d+(?:\.\d+)?)(Ki|Mi|Gi|Ti)$/.exec(memory.trim());
  if (!match) return null;
  return Number(match[1]) * BINARY_UNIT_BYTES[match[2]];
}

/**
 * Whether `tier` is a smaller plan than `current`: bex-api refuses that change
 * (`keyvalue.ErrPlanDowngrade`, "a Key Value instance cannot move to a smaller
 * plan", w8/249), comparing the tiers' memory from the shared catalog.
 *
 * Comparing memory — the same field bex-api compares — keeps the dashboard's
 * rule on the API's data instead of assuming the catalog's list order. An
 * unparseable quantity or a current plan the catalog does not list leaves the
 * card selectable: bex-api still refuses the change, and the UI never blocks a
 * legal one on a guess.
 */
export function isPlanDowngrade(
  tier: Pick<KeyValueInstanceTypeView, "memory">,
  current: Pick<KeyValueInstanceTypeView, "memory"> | undefined,
): boolean {
  if (!current) return false;
  const to = tierMemoryBytes(tier.memory);
  const from = tierMemoryBytes(current.memory);
  return to !== null && from !== null && to < from;
}
