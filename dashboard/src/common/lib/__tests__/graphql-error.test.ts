import { describe, it, expect } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import {
  conflictOrGenericMessage,
  hasGraphQLErrorCode,
  isNameConflictError,
  planLimitExtensions,
  refusalReason,
} from "@/common/lib/graphql-error";

/**
 * Build the kind of error Apollo throws when a GraphQL mutation returns errors.
 * CombinedGraphQLErrors is branded in its constructor so .is() recognises it.
 */
function gqlError(extensions: Record<string, unknown>) {
  return new CombinedGraphQLErrors({
    data: null,
    errors: [{ message: "backend error", extensions }],
  });
}

describe("planLimitExtensions", () => {
  it("returns plan+limit when code is PLAN_LIMIT", () => {
    const err = gqlError({ code: "PLAN_LIMIT", plan: "hobby", limit: 1 });
    expect(planLimitExtensions(err)).toEqual({ plan: "hobby", limit: 1 });
  });

  it("returns null for a different code even when message contains 'plan'", () => {
    // This test is the key proof: the old code keyed on message.includes("plan");
    // planLimitExtensions must ignore that and return null for wrong codes.
    const err = gqlError({ code: "RATE_LIMITED" });
    expect(planLimitExtensions(err)).toBeNull();
  });

  it("returns null when extensions carry no code", () => {
    const err = gqlError({ message: "the plan is full" });
    expect(planLimitExtensions(err)).toBeNull();
  });

  it("returns null for a plain Error (not a GraphQL error)", () => {
    expect(planLimitExtensions(new Error("plan not supported"))).toBeNull();
  });

  it("returns null for non-error values", () => {
    expect(planLimitExtensions(null)).toBeNull();
    expect(planLimitExtensions(undefined)).toBeNull();
    expect(planLimitExtensions("plan error")).toBeNull();
  });

  it("coerces missing params to safe defaults", () => {
    const err = gqlError({ code: "PLAN_LIMIT" });
    expect(planLimitExtensions(err)).toEqual({ plan: "", limit: 0 });
  });
});

describe("hasGraphQLErrorCode", () => {
  it("matches a structured code in any combined GraphQL error", () => {
    expect(
      hasGraphQLErrorCode(
        gqlError({ code: "PAYMENT_REQUIRED" }),
        "PAYMENT_REQUIRED",
      ),
    ).toBe(true);
  });

  it("does not key off error-message text", () => {
    expect(
      hasGraphQLErrorCode(new Error("PAYMENT_REQUIRED"), "PAYMENT_REQUIRED"),
    ).toBe(false);
    expect(
      hasGraphQLErrorCode(gqlError({ code: "FORBIDDEN" }), "PAYMENT_REQUIRED"),
    ).toBe(false);
  });
});

// w6/m49: a duplicate-name create conflict is keyed on extensions.code
// (core.NewConflictError, "CONFLICT") — not per-type message matching, so a
// backend copy change (or a third resource type's differently-worded message)
// can't silently stop a create-form's conflict handling from firing.
describe("isNameConflictError", () => {
  it("matches the CONFLICT code regardless of message wording", () => {
    expect(
      isNameConflictError(
        gqlError({ code: "CONFLICT" /* keyvalue's wording */ }),
      ),
    ).toBe(true);
    expect(isNameConflictError(gqlError({ code: "CONFLICT" }))).toBe(true);
  });

  it("does not match a different code, even one containing conflict-like text", () => {
    expect(isNameConflictError(gqlError({ code: "PLAN_LIMIT" }))).toBe(false);
    expect(isNameConflictError(new Error("already exists"))).toBe(false);
  });

  it("returns false for non-error values", () => {
    expect(isNameConflictError(null)).toBe(false);
    expect(isNameConflictError(undefined)).toBe(false);
  });
});

// w6/m49/t008: the four `use-create-*` hooks each wrote the identical
// isNameConflictError/refusalReason branch, so it graduated here.
describe("conflictOrGenericMessage", () => {
  it("returns the backend's specific reason on a name conflict", () => {
    const err = new CombinedGraphQLErrors({
      data: null,
      errors: [
        {
          message: 'a project named "acme" already exists in this workspace',
          extensions: { code: "CONFLICT" },
        },
      ],
    });
    expect(conflictOrGenericMessage(err, "generic fallback")).toBe(
      'A project named "acme" already exists in this workspace',
    );
  });

  it("returns the caller's generic message for a non-conflict error", () => {
    expect(
      conflictOrGenericMessage(new Error("network error"), "generic fallback"),
    ).toBe("generic fallback");
  });
});

// w1/m81 + w1/m82 — both the custom-domain and member-invite dialogs show the
// server's own reason instead of generic copy, so the prefix-stripping lives
// here once rather than being re-derived (and re-drifted) per feature.
describe("refusalReason", () => {
  it("strips the transport prefix and capitalizes the server's reason", () => {
    expect(
      refusalReason(
        new Error('bad request: wildcard hostnames are not allowed: "*.x.com"'),
      ),
    ).toBe('Wildcard hostnames are not allowed: "*.x.com"');
    expect(
      refusalReason(
        new Error("GraphQL error: bad request: invalid email address"),
      ),
    ).toBe("Invalid email address");
  });

  it("reads the first error of a combined GraphQL response", () => {
    const err = new CombinedGraphQLErrors(
      { data: null, errors: [{ message: "boss@x.com is already a member" }] },
      [{ message: "boss@x.com is already a member" }],
    );
    expect(refusalReason(err)).toBe("Boss@x.com is already a member");
  });

  it("returns empty when there is no usable message, so callers keep their generic copy", () => {
    expect(refusalReason(new Error(""))).toBe("");
    expect(refusalReason("not an error")).toBe("");
    expect(refusalReason(undefined)).toBe("");
  });
});
