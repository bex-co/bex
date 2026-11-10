/**
 * Browser-safe validation for canonical IPv4/IPv6 CIDR input, held to the
 * grammar of the server's netip.ParsePrefix: a prefix length with a leading
 * zero (`/024`, `/08`, `/00`) is refused, only a bare `/0` may start with 0
 * (lego/backend/internal/core/cidr.go; w21/038).
 */
export function isValidCIDR(value: string): boolean {
  const parts = value.trim().split("/");
  if (parts.length !== 2) return false;
  const [address, prefixText] = parts;
  if (!address || !/^\d+$/.test(prefixText)) return false;
  if (prefixText.length > 1 && prefixText.startsWith("0")) return false;
  const prefix = Number(prefixText);

  if (address.includes(":")) {
    if (prefix < 0 || prefix > 128) return false;
    try {
      const parsed = new URL(`http://[${address}]/`);
      return parsed.hostname.startsWith("[") && parsed.hostname.endsWith("]");
    } catch {
      return false;
    }
  }

  if (prefix < 0 || prefix > 32) return false;
  const octets = address.split(".");
  return (
    octets.length === 4 &&
    octets.every(
      (octet) =>
        /^\d{1,3}$/.test(octet) &&
        !(octet.length > 1 && octet.startsWith("0")) &&
        Number(octet) <= 255,
    )
  );
}
