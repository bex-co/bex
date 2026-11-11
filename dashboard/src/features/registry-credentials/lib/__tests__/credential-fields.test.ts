import { describe, expect, it } from "vitest";
import {
  authTokenError,
  canonicalRegistryHost,
  registryHostError,
  usernameError,
} from "../credential-fields";

// w21/053: the host and field vectors are the server's
// (lego/backend/internal/registrycreds TestCredentialDialogVectors pins the same
// table against canonicalRegistryHost and validateCredentialFields).
describe("registryHostError", () => {
  it.each([
    ["ghcr.io", "ghcr.io"],
    ["registry.example.com:5000", "registry.example.com:5000"],
    ["localhost:5000", "localhost:5000"],
    ["localhost", "localhost"],
    // What the server drops or folds: the scheme, trailing slashes, case.
    ["https://GHCR.IO/", "ghcr.io"],
    ["HTTP://ghcr.io///", "ghcr.io"],
    ["ghcr.io/", "ghcr.io"],
    ["  ghcr.io  ", "ghcr.io"],
    ["docker.io", "docker.io"],
    ["index.docker.io", "docker.io"],
    ["registry-1.docker.io", "docker.io"],
    ["Registry.Hub.Docker.Com", "docker.io"],
    ["a-b.c-d.example.org:65535", "a-b.c-d.example.org:65535"],
  ])("accepts %j as %j", (raw, canonical) => {
    expect(canonicalRegistryHost(raw)).toBe(canonical);
    expect(registryHostError(raw)).toBeNull();
  });

  it.each([
    "ghcr.io/qa-owner/qa-image",
    "my registry.io",
    "https://ghcr.io/owner",
    "ghcr.io?x=1",
    "user@ghcr.io",
    "ghcr.io:",
    "ghcr.io:123456",
    "ghcr.io:abc",
    "-ghcr.io",
    "ghcr-.io",
    "ghcr..io",
    ".ghcr.io",
    "bücher.de",
    "https://",
  ])("refuses %j as not a bare registry host", (raw) => {
    expect(registryHostError(raw)).toBe("registryCredentials.hostInvalid");
  });

  it("is quiet for a blank host (the dialog asks for it itself)", () => {
    expect(registryHostError("")).toBeNull();
    expect(registryHostError("   ")).toBeNull();
  });

  it("counts the canonical host in bytes", () => {
    const fits = `${"a".repeat(250)}.io`; // 253 bytes
    expect(fits).toHaveLength(253);
    expect(registryHostError(fits)).toBeNull();
    expect(registryHostError(`${fits}x`)).toBe(
      "registryCredentials.hostTooLong",
    );
    expect(registryHostError(`${"a".repeat(300)}.io`)).toBe(
      "registryCredentials.hostTooLong",
    );
    // The scheme and slash are dropped before the count, as the server does.
    expect(registryHostError(`https://${fits}/`)).toBeNull();
  });
});

describe("usernameError", () => {
  it.each(["alice", "alice@example.com", "中文用户", "a b", ""])(
    "accepts %j",
    (raw) => {
      expect(usernameError(raw)).toBeNull();
    },
  );

  it.each(["qa\tuser", "a\nb", "a\u0000b", "a\u007fb", "a\u0085b"])(
    "refuses a control character in %j",
    (raw) => {
      expect(usernameError(raw)).toBe("registryCredentials.usernameControl");
    },
  );

  it("drops surrounding whitespace, as the dialog sends it", () => {
    expect(usernameError("  alice\t")).toBeNull();
  });

  it("counts bytes, not characters", () => {
    expect(usernameError("u".repeat(253))).toBeNull();
    expect(usernameError("u".repeat(254))).toBe(
      "registryCredentials.usernameTooLong",
    );
    // 85 Chinese characters are 255 bytes.
    expect(usernameError("中".repeat(84))).toBeNull();
    expect(usernameError("中".repeat(85))).toBe(
      "registryCredentials.usernameTooLong",
    );
  });
});

describe("authTokenError", () => {
  it("allows up to 64 KiB", () => {
    expect(authTokenError("t".repeat(64 * 1024))).toBeNull();
    expect(authTokenError("t".repeat(64 * 1024 + 1))).toBe(
      "registryCredentials.authTokenTooLong",
    );
    expect(authTokenError("")).toBeNull();
  });
});
