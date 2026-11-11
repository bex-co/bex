export interface IPAllowListEntryDraft {
  cidrBlock: string;
  description: string;
}

export function ipAllowListEntryKey(entries: IPAllowListEntryDraft[]) {
  return JSON.stringify(entries);
}

const ALLOW_ALL_CIDRS = ["0.0.0.0/0", "::/0"];

/**
 * The allow-all rule a service reads when nobody restricted it, and the write
 * that opens one again (bex-api's `core.UnrestrictedAllowList`, w8/m157). An
 * EMPTY list is the opposite: it blocks every public request.
 */
export const ALLOW_ALL_ENTRIES: IPAllowListEntryDraft[] = ALLOW_ALL_CIDRS.map(
  (cidrBlock) => ({ cidrBlock, description: "" }),
);

/**
 * Whether `entries` are exactly the allow-all rule, which is what a service
 * with no saved rule reads. A lone `0.0.0.0/0` saved before w8/m157 is not:
 * it still refuses IPv6 sources until the allow-all rule is written over it.
 */
export function isAllowAllRule(entries: IPAllowListEntryDraft[]) {
  return (
    entries.length === ALLOW_ALL_CIDRS.length &&
    ALLOW_ALL_CIDRS.every((cidr) =>
      entries.some((entry) => entry.cidrBlock.trim() === cidr),
    )
  );
}
