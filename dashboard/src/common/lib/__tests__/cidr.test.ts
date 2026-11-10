import { describe, expect, it } from "vitest";
import { isValidCIDR } from "@/common/lib/cidr";

describe("isValidCIDR", () => {
  it.each([
    "0.0.0.0/0",
    "203.0.113.0/24",
    "255.255.255.255/32",
    "2001:db8::/32",
    "::1/128",
    "::/0",
    "10.0.0.0/8",
  ])("accepts %s", (value) => expect(isValidCIDR(value)).toBe(true));

  it.each([
    "",
    "203.0.113.0",
    "203.0.113.0/33",
    "256.0.0.1/24",
    "01.2.3.4/24",
    "2001:db8::/129",
    "not-an-ip/24",
    // w21/038: a zero-padded prefix is refused by netip.ParsePrefix, so by us.
    "10.0.0.0/024",
    "10.0.0.0/08",
    "10.0.0.0/00",
    "::1/0128",
    "2001:db8::/032",
  ])("rejects %s", (value) => expect(isValidCIDR(value)).toBe(false));
});
