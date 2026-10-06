import { describe, it, expect } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { classifyAddError } from "@/features/services/hooks/use-custom-domains";

describe("classifyAddError", () => {
  // bex-api's coded refusals (w5/m118): the dialog keys on the code, so the
  // server's wording can change without the localized line going missing.
  const coded = (code: string, message: string) =>
    new CombinedGraphQLErrors({
      errors: [{ message, extensions: { code } }],
    } as never);

  it("maps the two coded refusals to their localized keys", () => {
    expect(
      classifyAddError(
        coded(
          "CUSTOM_DOMAIN_IN_USE",
          "this domain already exists on another site",
        ),
      ),
    ).toEqual({ key: "services.domainAddConflict" });
    expect(
      classifyAddError(
        coded(
          "CUSTOM_DOMAIN_RESERVED",
          '"api.onbex.co" is a reserved platform hostname',
        ),
      ),
    ).toEqual({ key: "services.domainAddReserved" });
  });

  it("decides by the code, not the wording", () => {
    // The old sentinel text with no code is just another refusal now.
    expect(
      classifyAddError(new Error("host already exists on another site")),
    ).toEqual({ detail: "Host already exists on another site" });
    expect(
      classifyAddError(coded("CUSTOM_DOMAIN_IN_USE", "reworded upstream")),
    ).toEqual({ key: "services.domainAddConflict" });
  });

  it("surfaces the server's own reason for any other refusal (strips the bad-request prefix)", () => {
    // The wildcard case the QA walk hit: coded CUSTOM_DOMAIN_INVALID but not
    // given its own line, so the dialog must show *why* — the server's
    // message — not a generic failure.
    expect(
      classifyAddError(
        coded(
          "CUSTOM_DOMAIN_INVALID",
          'wildcard hostnames are not allowed: "*.x.com"',
        ),
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
