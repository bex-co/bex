import type { CustomDomainView } from "@/features/services/types";

/**
 * DNS zone Host/Name fields are relative to (w4/092). Derived from the traffic
 * record's relative name when present; otherwise from an apex domain name.
 */
export function dnsInstructionZone(domain: CustomDomainView): string {
  const traffic = domain.dnsRecord?.name;
  if (traffic && traffic !== "@") {
    const prefix = `${traffic}.`;
    if (domain.name.startsWith(prefix)) {
      return domain.name.slice(prefix.length);
    }
  }
  if (domain.domainType === "apex") {
    return domain.name;
  }
  const labels = domain.name.split(".");
  if (labels.length > 2) {
    return labels.slice(-2).join(".");
  }
  return domain.name;
}
