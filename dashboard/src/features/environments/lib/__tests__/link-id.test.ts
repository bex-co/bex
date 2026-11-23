import { describe, expect, it } from "vitest";
import { canonicalEnvironmentLinkId, inEnvironment } from "../link-id";

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

// w22/m603: a sandbox group names its environment; the card matches it to
// the environment whichever id spelling either side carries.
describe("inEnvironment", () => {
  const evm = "evm-davm6imde41s73canq0g";
  const env = "env-davm6imde41s73canq0g";

  it("matches the same environment in either spelling", () => {
    expect(inEnvironment(evm, evm)).toBe(true);
    expect(inEnvironment(env, evm)).toBe(true);
    expect(inEnvironment(evm, env)).toBe(true);
  });

  it("matches no environment for none or another", () => {
    expect(inEnvironment(null, evm)).toBe(false);
    expect(inEnvironment(undefined, evm)).toBe(false);
    expect(inEnvironment("evm-aaaaaaaaaaaaaaaaaaaa", evm)).toBe(false);
  });
});
