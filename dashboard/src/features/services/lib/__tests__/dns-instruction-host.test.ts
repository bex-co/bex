import { describe, it, expect } from "vitest";
import type { CustomDomainView } from "@/features/services/types";
import { dnsInstructionZone } from "@/features/services/lib/dns-instruction-host";

const base: CustomDomainView = {
  name: "api.example.com",
  domainType: "subdomain",
  dnsVerified: false,
  certificateVerified: false,
  active: false,
  redirectForName: null,
  certificateReason: null,
  dnsRecord: { type: "CNAME", name: "api", value: "web.onbex.co" },
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
});
