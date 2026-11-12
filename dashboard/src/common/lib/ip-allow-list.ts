export interface IPAllowListEntryDraft {
  cidrBlock: string;
  description: string;
}

export function ipAllowListEntryKey(entries: IPAllowListEntryDraft[]) {
  return JSON.stringify(entries);
}

const ALLOW_ALL_CIDRS = ["0.0.0.0/0", "::/0"];

/**
 * The rule a service reads when nobody restricted it, and the write that opens
 * one again: Render's default, `0.0.0.0/0` "everywhere" (bex-api's
 * `core.ServiceDefaultAllowList`, w8/284). An EMPTY list is the opposite: it
 * blocks every public request.
 */
export const ALLOW_ALL_ENTRIES: IPAllowListEntryDraft[] = [
  { cidrBlock: "0.0.0.0/0", description: "everywhere" },
];

/**
 * Whether a service's `entries` restrict nothing: every rule an all-addresses
 * range (`0.0.0.0/0` or `::/0`), whatever its description, and one of them
 * `0.0.0.0/0`. A service is reached over IPv4 only, so `::/0` alone blocks
 * every client. bex-api's `core.RestrictsServiceInbound` draws the same line.
 */
export function isAllowAllRule(entries: IPAllowListEntryDraft[]) {
  return (
    entries.some((entry) => entry.cidrBlock.trim() === "0.0.0.0/0") &&
    entries.every((entry) => ALLOW_ALL_CIDRS.includes(entry.cidrBlock.trim()))
  );
}
