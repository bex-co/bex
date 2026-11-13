import { describe, expect, it } from "vitest";
import {
  SECRET_MASK,
  connectionURLPassword,
  maskSecret,
} from "@/features/databases/lib/mask-secret";

describe("connectionURLPassword", () => {
  it("reads the password as the URL spells it, so maskSecret finds it", () => {
    const url = "postgresql://user:p%40ss@host:5432/db";
    expect(connectionURLPassword(url)).toBe("p%40ss");
    expect(maskSecret(url, connectionURLPassword(url))).toBe(
      `postgresql://user:${SECRET_MASK}@host:5432/db`,
    );
  });

  it("is empty for a URL without a password or one that does not parse", () => {
    expect(connectionURLPassword("postgresql://user@host/db")).toBe("");
    expect(connectionURLPassword("not a url")).toBe("");
    expect(maskSecret("postgresql://user@host/db", "")).toBe(
      "postgresql://user@host/db",
    );
  });
});
