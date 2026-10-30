import { describe, expect, it } from "vitest";
import { slugify } from "@/features/workspaces/lib/slug";
import { isValidDnsLabel } from "@/common/lib/utils/dns-label";

describe("slugify (w3/m246)", () => {
  it.each([
    ["Tian Pan's Workspace", "tian-pan-s-workspace"],
    ["  Acme  Prod  ", "acme-prod"],
    ["Café Ünïcode", "cafe-unicode"],
    ["---", ""],
    ["a".repeat(40), "a".repeat(30)],
    ["abcdefghijklmnopqrstuvwxyz abc def", "abcdefghijklmnopqrstuvwxyz-abc"],
  ])("%j suggests %j", (name, slug) => {
    expect(slugify(name)).toBe(slug);
  });

  it("only ever suggests a valid slug or nothing", () => {
    for (const name of [
      "Tian Pan's Workspace",
      "x y-",
      "中文 名称 ok",
      "9 lives",
    ]) {
      const slug = slugify(name);
      expect(slug === "" || isValidDnsLabel(slug)).toBe(true);
    }
  });
});
