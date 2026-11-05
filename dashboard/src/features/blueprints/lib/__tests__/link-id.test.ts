import { describe, expect, it } from "vitest";
import { canonicalBlueprintLinkId } from "../link-id";

// Shape edge cases live with the shared helper (common/lib/__tests__/link-id).
describe("canonicalBlueprintLinkId", () => {
  it("converts the durable blp- shape to the public exs- id", () => {
    expect(canonicalBlueprintLinkId("blp-db136288mmqc73d4hpug")).toBe(
      "exs-db136288mmqc73d4hpug",
    );
  });

  it("leaves env- and short names untouched", () => {
    expect(canonicalBlueprintLinkId("env-db136288mmqc73d4hpug")).toBe(
      "env-db136288mmqc73d4hpug",
    );
    expect(canonicalBlueprintLinkId("blp-abc123")).toBe("blp-abc123");
  });
});
