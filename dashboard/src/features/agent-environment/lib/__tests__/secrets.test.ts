import { describe, expect, it } from "vitest";
import {
  MAX_SECRET_VALUE_BYTES,
  RESERVED_NAMES,
  RESERVED_PREFIXES,
  SECRET_NAME_PATTERN,
  normalizeSecretRepo,
  secretAppliesByScope,
  secretNameProblem,
  secretValueProblem,
  toSecretScope,
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

describe("secret scopes (ADR098 D9)", () => {
  it("reads an unknown wire scope as explicit, the narrowest", () => {
    expect(toSecretScope("workspace")).toBe("workspace");
    expect(toSecretScope("repo")).toBe("repo");
    expect(toSecretScope("explicit")).toBe("explicit");
    expect(toSecretScope("organization")).toBe("explicit");
  });

  // The server's agentsession.NormalizeRepository, which a repo scope and a
  // session's repository both pass through.
  it("normalizes a repository as the server stores it", () => {
    expect(normalizeSecretRepo("Acme/API")).toBe("acme/api");
    expect(normalizeSecretRepo(" /acme/api.git ")).toBe("acme/api");
    for (const bad of ["", "acme", "acme/", "/api", "a/b/c", "a/..", "a b/c"]) {
      expect(normalizeSecretRepo(bad)).toBeNull();
    }
  });

  it("applies a workspace scope everywhere and a repo scope on its repository", () => {
    const workspace = { name: "A", scope: "workspace", repo: null } as const;
    const repo = { name: "B", scope: "repo", repo: "acme/api" } as const;
    const explicit = { name: "C", scope: "explicit", repo: null } as const;
    expect(secretAppliesByScope(workspace, null)).toBe(true);
    expect(secretAppliesByScope(repo, "ACME/Api")).toBe(true);
    expect(secretAppliesByScope(repo, "acme/web")).toBe(false);
    expect(secretAppliesByScope(repo, null)).toBe(false);
    expect(secretAppliesByScope(explicit, "acme/api")).toBe(false);
  });
});
