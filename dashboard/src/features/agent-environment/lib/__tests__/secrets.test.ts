import { describe, expect, it } from "vitest";
import {
  MAX_SECRET_VALUE_BYTES,
  RESERVED_NAMES,
  RESERVED_PREFIXES,
  SECRET_NAME_PATTERN,
  secretNameProblem,
  secretValueProblem,
} from "@/features/agent-environment/lib/secrets";
import { goRegexp, goStringSlice } from "@/test/go-source";

// The naming rules are a client mirror of bex-api's; read the server's from
// its source so a name the server reserves cannot slip past the form.
describe("secret naming matches bex-api", () => {
  const source = "lego/backend/internal/agentsessions/secrets.go";
  it("reserves the same names and prefixes", () => {
    expect([...RESERVED_NAMES].sort()).toEqual(
      goStringSlice(source, "reservedSecretNames").sort(),
    );
    expect([...RESERVED_PREFIXES].sort()).toEqual(
      goStringSlice(source, "reservedSecretPrefixes").sort(),
    );
  });
  it("uses the same name pattern", () => {
    expect(SECRET_NAME_PATTERN.source).toBe(
      goRegexp(source, "secretNamePattern"),
    );
  });
});

describe("secret naming (ADR098 D3)", () => {
  it("accepts env var names", () => {
    for (const name of ["STRIPE_KEY", "_private", "a1", "DATABASE_URL"]) {
      expect(secretNameProblem(name)).toBeNull();
    }
  });

  it("refuses malformed names", () => {
    for (const name of ["", "1A", "has-dash", "a.b", "x".repeat(129)]) {
      expect(secretNameProblem(name)).toBe(
        "agentEnvironment.secrets.nameInvalid",
      );
    }
  });

  it("refuses platform-reserved names in any case", () => {
    for (const name of [
      "BEX_X",
      "anthropic_api_key",
      "PATH",
      "https_proxy",
      "GIT_ASKPASS",
    ]) {
      expect(secretNameProblem(name)).toBe(
        "agentEnvironment.secrets.nameReserved",
      );
    }
  });

  it("bounds values by bytes, not characters", () => {
    expect(secretValueProblem("")).toBe(
      "agentEnvironment.secrets.valueRequired",
    );
    expect(secretValueProblem("a\0b")).toBe(
      "agentEnvironment.secrets.valueInvalid",
    );
    expect(
      secretValueProblem("é".repeat(MAX_SECRET_VALUE_BYTES / 2)),
    ).toBeNull();
    expect(secretValueProblem("é".repeat(MAX_SECRET_VALUE_BYTES / 2 + 1))).toBe(
      "agentEnvironment.secrets.valueInvalid",
    );
  });
});
