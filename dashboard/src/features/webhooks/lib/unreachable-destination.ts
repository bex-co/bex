/**
 * Whether a webhook Destination URL names a host bex can never deliver to:
 * the literal-host rule of the delivery guard (lego/types/netutil/ssrf.go
 * `UnsafeOriginIP`) and of the static-site rewrite check (apps/service.go
 * `validatePublicRewriteHost`) — a loopback, private, link-local, multicast,
 * unspecified or special-purpose address, a single-label name, or a name under
 * a local, internal or cluster suffix. A name that merely resolves to such an
 * address is caught when the delivery dials it, so it is not flagged here.
 *
 * It only informs: Render accepts any URL and reports a failed delivery in its
 * list, so the form notes the problem and still saves (w21/046). The vectors in
 * `__tests__/unreachable-destination.test.ts` are the ones the Go guard pins.
 */
const LOCAL_SUFFIXES = [
  "localhost",
  "local",
  "internal",
  "lan",
  "svc",
  "cluster.local",
  "home.arpa",
];

// IPv4 prefixes UnsafeOriginIP refuses: the net.IP predicates (loopback,
// private, link-local, multicast, unspecified) and its special-purpose list.
const V4_PREFIXES: Array<[string, number]> = [
  ["0.0.0.0", 8],
  ["10.0.0.0", 8],
  ["100.64.0.0", 10],
  ["127.0.0.0", 8],
  ["169.254.0.0", 16],
  ["172.16.0.0", 12],
  ["192.0.0.0", 24],
  ["192.0.2.0", 24],
  ["192.168.0.0", 16],
  ["198.18.0.0", 15],
  ["198.51.100.0", 24],
  ["203.0.113.0", 24],
  ["224.0.0.0", 4],
  ["240.0.0.0", 4],
];

function v4ToNumber(parts: number[]): number {
  return (
    ((parts[0] << 24) | (parts[1] << 16) | (parts[2] << 8) | parts[3]) >>> 0
  );
}

function parseV4(host: string): number[] | null {
  const parts = host.split(".");
  if (parts.length !== 4) return null;
  const numbers = parts.map((part) => (/^\d{1,3}$/.test(part) ? +part : NaN));
  return numbers.every((n) => n >= 0 && n <= 255) ? numbers : null;
}

function unsafeV4(parts: number[]): boolean {
  const value = v4ToNumber(parts);
  return V4_PREFIXES.some(([base, bits]) => {
    const mask = bits === 0 ? 0 : (~0 << (32 - bits)) >>> 0;
    return (value & mask) >>> 0 === (v4ToNumber(parseV4(base)!) & mask) >>> 0;
  });
}

/** Eight 16-bit groups of an IPv6 literal (without brackets), or null. */
function parseV6(host: string): number[] | null {
  let text = host;
  // A dotted IPv4 tail (::ffff:127.0.0.1) is two groups.
  const tail = /:(\d+\.\d+\.\d+\.\d+)$/.exec(text);
  if (tail) {
    const v4 = parseV4(tail[1]);
    if (!v4) return null;
    text =
      text.slice(0, tail.index) +
      `:${((v4[0] << 8) | v4[1]).toString(16)}:${((v4[2] << 8) | v4[3]).toString(16)}`;
  }
  const halves = text.split("::");
  if (halves.length > 2) return null;
  const toGroups = (side: string) => (side === "" ? [] : side.split(":"));
  const head = toGroups(halves[0]);
  const rest = halves.length === 2 ? toGroups(halves[1]) : [];
  if (halves.length === 1 && head.length !== 8) return null;
  if (halves.length === 2 && head.length + rest.length > 7) return null;
  const groups = [
    ...head,
    ...Array(halves.length === 2 ? 8 - head.length - rest.length : 0).fill("0"),
    ...rest,
  ];
  if (groups.length !== 8) return null;
  const numbers = groups.map((g) =>
    /^[0-9a-f]{1,4}$/i.test(g) ? parseInt(g, 16) : NaN,
  );
  return numbers.every((n) => !Number.isNaN(n)) ? numbers : null;
}

function unsafeV6(groups: number[]): boolean {
  const [a, b, c, d, e, f, g, h] = groups;
  const zeros = (...xs: number[]) => xs.every((x) => x === 0);
  if (zeros(a, b, c, d, e, f, g) && (h === 0 || h === 1)) return true; // :: and ::1
  if ((a & 0xffc0) === 0xfe80) return true; // fe80::/10 link-local
  if ((a & 0xfe00) === 0xfc00) return true; // fc00::/7 unique local
  if ((a & 0xff00) === 0xff00) return true; // ff00::/8 multicast
  if (a === 0x64 && b === 0xff9b && zeros(c, d, e, f)) return true; // 64:ff9b::/96 NAT64
  if (a === 0x100 && zeros(b, c, d)) return true; // 100::/64 discard-only
  if (a === 0x2001 && b === 0) return true; // 2001::/32 Teredo
  if (a === 0x2001 && b === 0xdb8) return true; // documentation
  if (a === 0x2002) return true; // 6to4
  // An IPv4-mapped answer is judged as the IPv4 address it carries.
  if (zeros(a, b, c, d, e) && f === 0xffff) {
    return unsafeV4([g >> 8, g & 0xff, h >> 8, h & 0xff]);
  }
  return false;
}

export function isUnreachableWebhookDestination(value: string): boolean {
  let hostname: string;
  try {
    hostname = new URL(value.trim()).hostname;
  } catch {
    return false; // not a URL: the form's own HTTPS message speaks
  }
  if (hostname === "") return false;
  if (hostname.startsWith("[") && hostname.endsWith("]")) {
    const groups = parseV6(hostname.slice(1, -1));
    return groups !== null && unsafeV6(groups);
  }
  const host = hostname.toLowerCase().replace(/\.$/, "");
  const v4 = parseV4(host);
  if (v4) return unsafeV4(v4);
  if (!host.includes(".")) return true;
  return LOCAL_SUFFIXES.some(
    (suffix) => host === suffix || host.endsWith(`.${suffix}`),
  );
}

/**
 * The delivery guard's refusal (lego/types/netutil/ssrf.go), as it reaches a
 * delivery's transport error: the destination resolved to an address bex does
 * not dial. A Go test pins the sentence.
 */
export function isBlockedDestinationError(transportError: string): boolean {
  return transportError.includes("destination resolves to a blocked address");
}
