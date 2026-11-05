import { describe, expect, it } from "vitest";
import { canonicalLinkId } from "../link-id";

describe("canonicalLinkId", () => {
  it.each([
    ["env", "evm", "env-davm6imde41s73canq0g", "evm-davm6imde41s73canq0g"],
    ["blp", "exs", "blp-db136288mmqc73d4hpug", "exs-db136288mmqc73d4hpug"],
  ])(
    "converts the historical %s- shape to the public %s- id",
    (legacy, publicPrefix, value, expected) => {
      expect(canonicalLinkId(value, legacy, publicPrefix)).toBe(expected);
    },
  );

  it.each([
    "evm-davm6imde41s73canq0g",
    "srv-davm6imde41s73canq0g",
    "env-production",
    "env-davm6imde41s73canq0",
    "env-davm6imde41s73canq0g0",
    "env-davm6imde41s73canq0w",
    "env-DAVM6IMDE41S73CANQ0G",
    "ENV-davm6imde41s73canq0g",
    " env-davm6imde41s73canq0g",
    "env-davm6imde41s73canq0g\n",
    "unassigned",
    "",
  ])(
    "does not rewrite names, canonical IDs, or unknown formats: %j",
    (value) => {
      expect(canonicalLinkId(value, "env", "evm")).toBe(value);
    },
  );
});
