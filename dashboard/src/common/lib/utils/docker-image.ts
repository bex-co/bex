// The Docker image rules bex-api enforces (lego/backend/internal/store
// ValidateImage), checked in the field before submit (w4/406): the reference
// grammar, the 512-byte cap, and no localhost or private/link-local registry
// host. The public/trusted-registry decision stays with the server, which
// resolves (or refuses) the registry itself.

export type DockerImageProblem = "malformed" | "tooLong" | "privateHost";

const MAX_IMAGE_BYTES = 512;

// distribution/reference grammar: a lowercase path of components, an optional
// registry domain, an optional tag, an optional digest.
const NAME_COMPONENT = String.raw`[a-z0-9]+(?:(?:[._]|__|-*)[a-z0-9]+)*`;
const DOMAIN_COMPONENT = String.raw`[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?`;
const DOMAIN = String.raw`(?:${DOMAIN_COMPONENT}(?:\.${DOMAIN_COMPONENT})*(?::[0-9]+)?|\[[0-9a-fA-F:.]+\](?::[0-9]+)?)`;
const TAG = String.raw`[\w][\w.-]{0,127}`;
const DIGEST = String.raw`[a-z0-9]+(?:[+._-][a-z0-9]+)*:[0-9a-f]{32,}`;
const IMAGE_REF_RE = new RegExp(
  String.raw`^(?:(${DOMAIN})/)?(${NAME_COMPONENT}(?:/${NAME_COMPONENT})*)(?::(${TAG}))?(?:@(${DIGEST}))?$`,
);

// Non-public IPv4 ranges (netutil.UnsafeOriginIP): loopback, RFC1918,
// link-local, CGNAT, the special-purpose registries, multicast, reserved.
const UNSAFE_IPV4: ReadonlyArray<readonly [number, number]> = [
  [0x00000000, 8], // 0.0.0.0/8 "this network"
  [0x0a000000, 8], // 10.0.0.0/8
  [0x64400000, 10], // 100.64.0.0/10 CGNAT
  [0x7f000000, 8], // 127.0.0.0/8 loopback
  [0xa9fe0000, 16], // 169.254.0.0/16 link-local
  [0xac100000, 12], // 172.16.0.0/12
  [0xc0000000, 24], // 192.0.0.0/24
  [0xc0000200, 24], // 192.0.2.0/24 TEST-NET-1
  [0xc0a80000, 16], // 192.168.0.0/16
  [0xc6120000, 15], // 198.18.0.0/15 benchmarking
  [0xc6336400, 24], // 198.51.100.0/24 TEST-NET-2
  [0xcb007100, 24], // 203.0.113.0/24 TEST-NET-3
  [0xe0000000, 4], // 224.0.0.0/4 multicast
  [0xf0000000, 4], // 240.0.0.0/4 reserved
];

function ipv4Value(host: string): number | null {
  const parts = host.split(".");
  if (parts.length !== 4) return null;
  let value = 0;
  for (const part of parts) {
    if (!/^(0|[1-9][0-9]{0,2})$/.test(part)) return null;
    const octet = Number(part);
    if (octet > 255) return null;
    value = value * 256 + octet;
  }
  return value;
}

function unsafeIPv4(host: string): boolean {
  const value = ipv4Value(host);
  if (value === null) return false;
  return UNSAFE_IPV4.some(([base, bits]) => {
    const shift = 32 - bits;
    return value >>> shift === base >>> shift;
  });
}

function unsafeIPv6(host: string): boolean {
  const h = host.toLowerCase();
  if (h === "::" || h === "::1") return true;
  // fe80::/10 link-local, fc00::/7 unique-local, ff00::/8 multicast.
  return /^fe[89ab]/.test(h) || /^f[cd]/.test(h) || h.startsWith("ff");
}

function unsafeRegistryHost(ref: string): boolean {
  const first = ref.split("/", 1)[0];
  // The first segment is a registry host only when it names one (a dot, a
  // port, localhost, or an IP literal); otherwise it is a Docker Hub path.
  const bracketed = first.startsWith("[");
  const isHost =
    bracketed ||
    first.includes(".") ||
    first.includes(":") ||
    first.toLowerCase() === "localhost";
  if (!isHost) return false;
  if (bracketed) {
    const host = first.slice(1, first.indexOf("]"));
    return unsafeIPv6(host) || unsafeIPv4(host);
  }
  const host = first.split(":", 1)[0].toLowerCase();
  if (host === "localhost") return true;
  return unsafeIPv4(host) || unsafeIPv6(host);
}

/**
 * The problem with a Docker image reference, or null when the field may
 * submit it. An empty value is not a problem here: the form's required check
 * owns that state, as it does for the name.
 */
export function dockerImageProblem(raw: string): DockerImageProblem | null {
  const value = raw.trim();
  if (value === "") return null;
  if (new TextEncoder().encode(value).length > MAX_IMAGE_BYTES)
    return "tooLong";
  if (!IMAGE_REF_RE.test(value)) return "malformed";
  if (unsafeRegistryHost(value)) return "privateHost";
  return null;
}

export function isValidDockerImageRef(raw: string): boolean {
  return raw.trim() !== "" && dockerImageProblem(raw) === null;
}
