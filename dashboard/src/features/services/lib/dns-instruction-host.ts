import type { CustomDomainView } from "@/features/services/types";

const OWNERSHIP_RELATIVE_HOST = "_bex-challenge";

/**
 * DNS zone Host/Name fields are relative to (w4/092). Derived from the traffic
 * record's relative name when present; otherwise from a legacy FQDN ownership
 * host or an apex domain name.
 */
export function dnsInstructionZone(domain: CustomDomainView): string {
  const traffic = domain.dnsRecord?.name;
  if (traffic && traffic !== "@") {
    const prefix = `${traffic}.`;
    if (domain.name.startsWith(prefix)) {
      return domain.name.slice(prefix.length);
    }
  }
  const ownership = domain.ownershipDnsRecord?.name;
  if (
    ownership &&
    ownership.startsWith(`${OWNERSHIP_RELATIVE_HOST}.`) &&
    ownership.length > OWNERSHIP_RELATIVE_HOST.length + 1
  ) {
    return ownership.slice(OWNERSHIP_RELATIVE_HOST.length + 1);
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

/**
 * Ownership TXT Host for the DNS panel — always relative to
 * {@link dnsInstructionZone}. Strips a legacy FQDN form so a dashboard build
 * ahead of the API still pastes correctly (w4/092).
 */
export function ownershipDnsHostForDisplay(recordName: string): string {
  if (recordName.startsWith(`${OWNERSHIP_RELATIVE_HOST}.`)) {
    return OWNERSHIP_RELATIVE_HOST;
  }
  return recordName;
}

/** Shared ownership-host key: relative host + zone (siblings must match both). */
export function ownershipHostKey(domain: CustomDomainView): string | null {
  const name = domain.ownershipDnsRecord?.name;
  if (!name) return null;
  return `${ownershipDnsHostForDisplay(name)}@${dnsInstructionZone(domain)}`;
}
