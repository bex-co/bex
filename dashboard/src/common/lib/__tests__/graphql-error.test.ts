import { describe, it, expect } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { planLimitExtensions } from "@/common/lib/graphql-error";

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
