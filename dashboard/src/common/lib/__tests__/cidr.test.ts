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
  ])("accepts %s", (value) => expect(isValidCIDR(value)).toBe(true));

  it.each([
    "",
    "203.0.113.0",
    "203.0.113.0/33",
    "256.0.0.1/24",
    "01.2.3.4/24",
    "2001:db8::/129",
    "not-an-ip/24",
  ])("rejects %s", (value) => expect(isValidCIDR(value)).toBe(false));
});
