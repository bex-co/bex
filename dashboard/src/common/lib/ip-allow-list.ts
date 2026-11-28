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

/**
 * An IPv4 range as Render's Networking view reads a rule: the base address
 * masked to its prefix, so `10.1.2.3/16` is `10.1.0.0/16`, the range the edge
 * matches. Render supports only IPv4 rules and its view skips any other entry
 * (its netmask library parses IPv4 alone); bex's service hosts publish no
 * AAAA record either, so an IPv6 range admits no client of theirs.
 */
interface IPv4Range {
  base: number;
  bits: number;
}

const IPV4_ADDRESS = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/;

/** A dotted-quad IPv4 address as a 32-bit unsigned number, or null. */
function parseIPv4Address(text: string): number | null {
  const match = IPV4_ADDRESS.exec(text.trim());
  if (!match) return null;
  let value = 0;
  for (const part of match.slice(1)) {
    if (part.length > 1 && part.startsWith("0")) return null;
    const octet = Number(part);
    if (octet > 255) return null;
    value = value * 256 + octet;
  }
  return value;
}

function ipv4Mask(bits: number): number {
  return bits === 0 ? 0 : (0xffffffff << (32 - bits)) >>> 0;
}

function parseIPv4Range(cidr: string): IPv4Range | null {
  const [address, prefix, ...rest] = cidr.trim().split("/");
  if (rest.length > 0 || prefix === undefined || !/^\d{1,2}$/.test(prefix)) {
    return null;
  }
  const bits = Number(prefix);
  const value = parseIPv4Address(address);
  if (value == null || bits > 32) return null;
  return { base: (value & ipv4Mask(bits)) >>> 0, bits };
}

function ipv4Ranges(entries: ReadonlyArray<IPAllowListEntryDraft>) {
  return entries.flatMap((entry) => {
    const range = parseIPv4Range(entry.cidrBlock);
    return range ? [range] : [];
  });
}

function rangeAdmits(range: IPv4Range, address: number) {
  return (address & ipv4Mask(range.bits)) >>> 0 === range.base;
}

function rangeContains(outer: IPv4Range, inner: IPv4Range) {
  return outer.bits <= inner.bits && rangeAdmits(outer, inner.base);
}

function formatIPv4Range({ base, bits }: IPv4Range) {
  return `${[base >>> 24, (base >>> 16) & 255, (base >>> 8) & 255, base & 255].join(".")}/${bits}`;
}

/**
 * The IPv4 ranges a source must fall in to pass every level of inbound IP
 * rules at once (Render: "an inbound IP must be allowed by each level's
 * rules"), as Render's Networking view counts them. Two CIDR ranges are
 * either disjoint or nested, so each pair across two levels leaves the
 * narrower range or nothing. An `undefined` level does not apply (a resource
 * outside any environment); an empty one allows nothing.
 */
export function inboundRangesAllowedByEveryLevel(
  levels: ReadonlyArray<ReadonlyArray<IPAllowListEntryDraft> | undefined>,
): string[] {
  const present = levels.filter((level) => level !== undefined);
  if (present.length === 0) return [];
  const [first, ...rest] = present.map(ipv4Ranges);
  const survivors = rest.reduce(
    (allowed, level) =>
      allowed.flatMap((a) =>
        level.flatMap((b) =>
          rangeContains(a, b) ? [b] : rangeContains(b, a) ? [a] : [],
        ),
      ),
    first,
  );
  return [...new Set(survivors.map(formatIPv4Range))];
}

/** A level of inbound IP rules above a resource's own (Render's "Affected by:"). */
export type InboundIPLevel = "workspace" | "environment";

/**
 * What Render's Validate IP address field says about `address`: nothing typed
 * yet, not an IPv4 address, allowed by every level, or refused, naming the
 * first level that refuses it (workspace, then environment, then the
 * resource's own rules, which the caller passes as the editor's draft).
 */
export type InboundAddressCheck =
  | { state: "empty" }
  | { state: "invalid" }
  | { state: "allowed" }
  | { state: "refused"; level: InboundIPLevel | "own" };

export function checkInboundAddress(
  address: string,
  levels: {
    workspace?: ReadonlyArray<IPAllowListEntryDraft>;
    environment?: ReadonlyArray<IPAllowListEntryDraft>;
    own: ReadonlyArray<IPAllowListEntryDraft>;
  },
): InboundAddressCheck {
  if (address.trim() === "") return { state: "empty" };
  const value = parseIPv4Address(address);
  if (value == null) return { state: "invalid" };
  const refuses = (entries: ReadonlyArray<IPAllowListEntryDraft> | undefined) =>
    entries !== undefined &&
    !ipv4Ranges(entries).some((range) => rangeAdmits(range, value));
  if (refuses(levels.workspace))
    return { state: "refused", level: "workspace" };
  if (refuses(levels.environment)) {
    return { state: "refused", level: "environment" };
  }
  if (refuses(levels.own)) return { state: "refused", level: "own" };
  return { state: "allowed" };
}
