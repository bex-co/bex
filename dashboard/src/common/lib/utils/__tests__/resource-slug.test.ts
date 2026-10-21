import { createHash } from "node:crypto";
import { describe, expect, it } from "vitest";
import {
  resourceSlug,
  shortResourceSlug,
} from "@/common/lib/utils/resource-slug";

// The first six hex digits of sha256(name), the suffix bex-api's
// blueprintResourceSlug appends to a slug over 30 characters.
function sum(name: string): string {
  return createHash("sha256").update(name, "utf8").digest("hex").slice(0, 6);
}

describe("resourceSlug (w5/m153, blueprintResourceSlug's rule)", () => {
  it.each([
    ["my-api", "my-api"],
    ["web-1", "web-1"],
    ["1worker", "1worker"],
    ["a".repeat(30), "a".repeat(30)],
    ["My API", "my-api"],
    ["My_API", "my-api"],
    ["  --Hello__World!!  ", "hello-world"],
    ["-invalid", "invalid"],
    ["web-", "web"],
    ["a  -  b", "a-b"],
    ["Café Ünïcode", "cafe-unicode"],
    ["QA 20261021 Ünï Form!", "qa-20261021-uni-form"],
    ["İstanbul", "istanbul"],
    ["Kube", "kube"],
    ["A".repeat(30), "a".repeat(30)],
  ])("maps %j to %j", async (name, want) => {
    expect(shortResourceSlug(name)).toBe(want);
    await expect(resourceSlug(name)).resolves.toBe(want);
  });

  it("gives a name with no Latin letters or digits its own stable resource name (w4/385)", async () => {
    const names = ["日本語のみ", "中文名", "🚀🚀", "!!!"];
    const slugs = new Set<string>();
    for (const name of names) {
      expect(shortResourceSlug(name)).toBeNull();
      const slug = await resourceSlug(name);
      expect(slug).toBe(`resource-${sum(name)}`);
      slugs.add(slug);
    }
    expect(slugs.size).toBe(names.length);
  });

  it("caps a slug over 30 characters at 23 plus a sha256 suffix of the original", async () => {
    const name = "Customer Billing Reconciliation Worker";
    expect(shortResourceSlug(name)).toBeNull();
    const slug = await resourceSlug(name);
    expect(slug).toBe(`customer-billing-reconc-${sum(name)}`);
    expect(slug).toHaveLength(30);
  });

  it("hashes a 31-character name, even one already lowercase", async () => {
    await expect(resourceSlug("a".repeat(31))).resolves.toBe(
      `${"a".repeat(23)}-${sum("a".repeat(31))}`,
    );
    await expect(resourceSlug("A".repeat(31))).resolves.toBe(
      `${"a".repeat(23)}-${sum("A".repeat(31))}`,
    );
  });

  it("trims a hyphen left at the cut before the suffix", async () => {
    const name = `${"x".repeat(22)} ${"y".repeat(20)}`;
    await expect(resourceSlug(name)).resolves.toBe(
      `${"x".repeat(22)}-${sum(name)}`,
    );
  });

  it("hashes the name's UTF-8 bytes", async () => {
    const name = `Ünïcode ${"service ".repeat(4)}名前`;
    await expect(resourceSlug(name)).resolves.toBe(
      `unicode-service-service-${sum(name)}`,
    );
  });
});
