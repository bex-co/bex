import { describe, it, expect } from "vitest";
import type { CustomDomainView } from "@/features/services/types";
import {
  dnsInstructionZone,
  ownershipDnsHostForDisplay,
  ownershipHostKey,
} from "@/features/services/lib/dns-instruction-host";

const base: CustomDomainView = {
  name: "api.example.com",
  domainType: "subdomain",
  ownershipVerified: false,
  verified: false,
  active: false,
  redirectForName: null,
  dnsRecord: { type: "CNAME", name: "api", value: "web.onbex.co" },
  ownershipDnsRecord: {
    type: "TXT",
    name: "_bex-challenge",
    value: "bex-domain-verification=x",
  },
};

describe("dns-instruction-host", () => {
  it("derives the zone from the traffic record's relative Host", () => {
    expect(dnsInstructionZone(base)).toBe("example.com");
  });

  it("uses the apex domain as the zone", () => {
    expect(
      dnsInstructionZone({
        ...base,
        name: "example.com",
        domainType: "apex",
        dnsRecord: { type: "ALIAS", name: "@", value: "web.onbex.co" },
      }),
    ).toBe("example.com");
  });

  it("strips a legacy FQDN ownership Host to the relative label", () => {
    expect(ownershipDnsHostForDisplay("_bex-challenge.example.com")).toBe(
      "_bex-challenge",
    );
    expect(ownershipDnsHostForDisplay("_bex-challenge")).toBe("_bex-challenge");
  });

  it("keys siblings by relative host and zone, not FQDN string equality", () => {
    const relative = ownershipHostKey(base);
    const legacy = ownershipHostKey({
      ...base,
      ownershipDnsRecord: {
        type: "TXT",
        name: "_bex-challenge.example.com",
        value: "bex-domain-verification=y",
      },
    });
    expect(relative).toBe("_bex-challenge@example.com");
    expect(legacy).toBe(relative);
  });
});
