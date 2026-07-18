/** Browser-safe validation for canonical IPv4/IPv6 CIDR input. */
export function isValidCIDR(value: string): boolean {
  const parts = value.trim().split("/");
  if (parts.length !== 2) return false;
  const [address, prefixText] = parts;
  if (!address || !/^\d+$/.test(prefixText)) return false;
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
