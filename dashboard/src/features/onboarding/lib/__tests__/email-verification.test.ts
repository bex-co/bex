import { describe, expect, it } from "vitest";
import type { Session } from "@ory/client-fetch";
import {
  emailVerificationPath,
  emailVerificationRequired,
  sessionTraitEmail,
} from "../email-verification";

const withIdentity = (identity: Record<string, unknown>) =>
  ({ id: "ses-1", identity }) as unknown as Session;

describe("emailVerificationPath", () => {
  it("carries a guarded deep link as next", () => {
    expect(emailVerificationPath("/services/new?type=web")).toBe(
      "/auth/verification?next=%2Fservices%2Fnew%3Ftype%3Dweb",
    );
  });

  it("drops an off-origin or empty next", () => {
    expect(emailVerificationPath("https://evil.example/")).toBe(
      "/auth/verification",
    );
    expect(emailVerificationPath("//evil.example")).toBe("/auth/verification");
    expect(emailVerificationPath("")).toBe("/auth/verification");
    expect(emailVerificationPath(undefined)).toBe("/auth/verification");
  });
});

describe("emailVerificationRequired", () => {
  const unverified = withIdentity({
    traits: { email: "dev@example.com" },
    verifiable_addresses: [{ value: "dev@example.com", verified: false }],
  });

  it("is true for a definitive unverified trait email", () => {
    expect(emailVerificationRequired(unverified)).toBe(true);
  });

  it("is false once the trait email's address is verified, ignoring case", () => {
    expect(
      emailVerificationRequired(
        withIdentity({
          traits: { email: "DEV@example.com" },
          verifiable_addresses: [{ value: "dev@EXAMPLE.com", verified: true }],
        }),
      ),
    ).toBe(false);
  });

  it("does not let a different verified address stand in for the trait email", () => {
    expect(
      emailVerificationRequired(
        withIdentity({
          traits: { email: "new@example.com" },
          verifiable_addresses: [{ value: "old@example.com", verified: true }],
        }),
      ),
    ).toBe(true);
  });

  it("requires verified === true, not a truthy status", () => {
    expect(
      emailVerificationRequired(
        withIdentity({
          traits: { email: "dev@example.com" },
          verifiable_addresses: [
            { value: "dev@example.com", verified: "true", status: "completed" },
          ],
        }),
      ),
    ).toBe(true);
  });

  it("fails open on indeterminate input", () => {
    expect(emailVerificationRequired(null)).toBe(false);
    expect(emailVerificationRequired(undefined)).toBe(false);
    expect(emailVerificationRequired(withIdentity({ traits: {} }))).toBe(false);
    expect(
      emailVerificationRequired(
        withIdentity({ traits: { email: "dev@example.com" } }),
      ),
    ).toBe(false);
  });
});

describe("sessionTraitEmail", () => {
  it("reads a non-empty string trait only", () => {
    expect(
      sessionTraitEmail(withIdentity({ traits: { email: "a@b.co" } })),
    ).toBe("a@b.co");
    expect(sessionTraitEmail(withIdentity({ traits: { email: "" } }))).toBe(
      undefined,
    );
    expect(sessionTraitEmail(withIdentity({ traits: { email: 7 } }))).toBe(
      undefined,
    );
    expect(sessionTraitEmail(null)).toBe(undefined);
  });
});
