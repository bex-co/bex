import { describe, it, expect } from "vitest";
import { isValidDnsLabel } from "../dns-label";
import { readRepoVectors, repeated } from "@/test/repo-vectors";

// The one table of resource names: appv1alpha1.ValidResourceName is its
// source of truth (lego/types/v1alpha1/resourcename_test.go), w5/m118.
const VECTORS = readRepoVectors<{
  name: string;
  repeat?: number;
  valid: boolean;
}>("lego/types/v1alpha1/testdata/resource-names.json");

describe("the shared resource-name vectors", () => {
  it.each(VECTORS.map((v) => [repeated(v.name, v.repeat), v.valid] as const))(
    "isValidDnsLabel(%j) is %s, the same as bex-api",
    (name, valid) => {
      expect(isValidDnsLabel(name)).toBe(valid);
    },
  );
});

// w4/m19: aligned with the backend's own rule (appv1alpha1.ValidResourceName)
// — digit-start allowed, 30-char cap, not the old letter-start/63-char rule.
describe("isValidDnsLabel", () => {
  it("accepts lowercase letters, digits, and interior hyphens", () => {
    expect(isValidDnsLabel("web")).toBe(true);
    expect(isValidDnsLabel("web-1")).toBe(true);
    expect(isValidDnsLabel("beancount-cms-v2")).toBe(true);
  });

  it("accepts a digit-start name (unlike the old letter-start rule)", () => {
    expect(isValidDnsLabel("1invalid")).toBe(true);
  });

  it("rejects a leading or trailing hyphen", () => {
    expect(isValidDnsLabel("-web")).toBe(false);
    expect(isValidDnsLabel("web-")).toBe(false);
  });

  it("rejects uppercase, underscores, and other invalid characters", () => {
    expect(isValidDnsLabel("Web")).toBe(false);
    expect(isValidDnsLabel("web_1")).toBe(false);
    expect(isValidDnsLabel("web.1")).toBe(false);
  });

  it("rejects empty", () => {
    expect(isValidDnsLabel("")).toBe(false);
  });

  it("enforces the 30-char cap, not the old 63-char one", () => {
    expect(isValidDnsLabel("a".repeat(30))).toBe(true);
    expect(isValidDnsLabel("a".repeat(31))).toBe(false);
    expect(isValidDnsLabel("a".repeat(63))).toBe(false);
  });
});
