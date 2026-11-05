import { describe, expect, it } from "vitest";
import { canonicalEnvironmentLinkId } from "../link-id";

// Shape edge cases live with the shared helper (common/lib/__tests__/link-id).
describe("canonicalEnvironmentLinkId", () => {
  it("converts the historical env- ID to the public evm- alias", () => {
    expect(canonicalEnvironmentLinkId("env-davm6imde41s73canq0g")).toBe(
      "evm-davm6imde41s73canq0g",
    );
  });

  it("leaves blp- and names untouched", () => {
    expect(canonicalEnvironmentLinkId("blp-davm6imde41s73canq0g")).toBe(
      "blp-davm6imde41s73canq0g",
    );
    expect(canonicalEnvironmentLinkId("env-production")).toBe("env-production");
  });
});
