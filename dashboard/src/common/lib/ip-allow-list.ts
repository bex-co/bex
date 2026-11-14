export interface IPAllowListEntryDraft {
  cidrBlock: string;
  description: string;
}

export function ipAllowListEntryKey(entries: IPAllowListEntryDraft[]) {
  return JSON.stringify(entries);
}

const ALLOW_ALL_CIDRS = ["0.0.0.0/0", "::/0"];

/**
 * A list as GraphQL reads it (`description` nullable) in the editor's draft
 * shape, or null when the response carried no list: not loaded is not "no
 * rules", since an empty list is deny-all wherever these rules apply.
 */
export function toIPAllowListDrafts(
  entries:
    | ReadonlyArray<{ cidrBlock: string; description: string | null } | null>
    | null
    | undefined,
): IPAllowListEntryDraft[] | null {
  if (entries == null) return null;
  return entries
    .filter((entry): entry is NonNullable<typeof entry> => entry != null)
    .map((entry) => ({
      cidrBlock: entry.cidrBlock,
      description: entry.description ?? "",
    }));
}

/**
 * Render's seeded allow-all pair for an environment or a workspace (bex-api's
 * `core.DefaultEnvironmentAllowList`): what one reads when nobody set rules,
 * and the reset every plan may write (w8/m137, w9/m388).
 */
export const ALLOW_ALL_PAIR_ENTRIES: IPAllowListEntryDraft[] = [
  { cidrBlock: "0.0.0.0/0", description: "Allow all (default)" },
  { cidrBlock: "::/0", description: "Allow all (default)" },
];

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
