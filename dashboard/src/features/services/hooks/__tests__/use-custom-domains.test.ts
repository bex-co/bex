import { describe, it, expect } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { classifyAddError } from "@/features/services/hooks/use-custom-domains";

describe("classifyAddError", () => {
  it("maps the two stable sentinels to their localized keys", () => {
    expect(
      classifyAddError(new Error("host already exists on another site")),
    ).toEqual({ key: "services.domainAddConflict" });
    expect(
      classifyAddError(new Error("that is a reserved platform hostname")),
    ).toEqual({ key: "services.domainAddReserved" });
  });

  it("surfaces the server's own reason for any other refusal (strips the bad-request prefix)", () => {
    // The wildcard case the QA walk hit: not a special-cased sentinel, so the
    // dialog must show *why* — the server's message — not a generic failure.
    expect(
      classifyAddError(
        new Error('bad request: wildcard hostnames are not allowed: "*.x.com"'),
      ),
    ).toEqual({ detail: 'Wildcard hostnames are not allowed: "*.x.com"' });

    // A different, unforeseen rejection also carries its own reason through.
    expect(
      classifyAddError(
        new Error("bad request: apex domains are not supported"),
      ),
    ).toEqual({ detail: "Apex domains are not supported" });
  });

  it("unwraps a GraphQL error result to the first error's message", () => {
    const err = new CombinedGraphQLErrors(
      { data: null, errors: [{ message: "bad request: something specific" }] },
      [{ message: "bad request: something specific" }],
    );
    expect(classifyAddError(err)).toEqual({ detail: "Something specific" });
  });

  it("returns nothing to classify when the message is empty (caller falls back to the generic key)", () => {
    expect(classifyAddError(new Error(""))).toEqual({});
    expect(classifyAddError("not an error")).toEqual({});
  });
});
