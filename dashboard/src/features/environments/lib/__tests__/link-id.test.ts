import { describe, expect, it } from "vitest";
import { canonicalEnvironmentLinkId } from "../link-id";

describe("canonicalEnvironmentLinkId", () => {
  it("converts the historical environment ID shape to the public alias", () => {
    expect(canonicalEnvironmentLinkId("env-davm6imde41s73canq0g")).toBe(
      "evm-davm6imde41s73canq0g",
    );
  });

  it.each([
    "evm-davm6imde41s73canq0g",
    "srv-davm6imde41s73canq0g",
    "env-production",
    "env-davm6imde41s73canq0",
    "env-davm6imde41s73canq0g0",
    "env-davm6imde41s73canq0w",
    "env-DAVM6IMDE41S73CANQ0G",
    " env-davm6imde41s73canq0g",
    "env-davm6imde41s73canq0g\n",
    "unassigned",
    "",
  ])(
    "does not rewrite names, canonical IDs, or unknown formats: %j",
    (value) => {
      expect(canonicalEnvironmentLinkId(value)).toBe(value);
    },
  );
});
