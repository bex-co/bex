import { describe, it, expect } from "vitest";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import {
  protectedConfirmationFromError,
  protectedServiceName,
} from "../protected-confirmation";

const REFUSAL = "PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED";

describe("protectedConfirmationFromError", () => {
  it("reads the phrase from the protected-environment refusal's extensions", () => {
    const err = codedGraphQLError(REFUSAL, {
      confirm: "sudo deploy service api",
      verb: "deploy",
      name: "api",
    });
    expect(protectedConfirmationFromError(err)).toBe("sudo deploy service api");
  });

  // The code decides, never the wording (w5/m128): the refusal's own text
  // without its code, and a Blueprint takeover sharing the confirm= convention
  // (blueprintTakeoverFromError classifies those, w5/m125), are not this
  // refusal.
  it("ignores a confirm= phrase that arrives without the code", () => {
    for (const message of [
      '"api" is a member of a protected environment; retry with confirm="sudo deploy service api" to deploy it',
      'service "web" is managed by blueprint blp-abc; retry with confirm="takeover blueprint blp-abc" to transfer ownership to this blueprint',
    ]) {
      expect(
        protectedConfirmationFromError(uncodedGraphQLError(message)),
      ).toBeNull();
    }
  });

  it("ignores a takeover's confirm, which is another code's", () => {
    const takeover = codedGraphQLError("BLUEPRINT_RESOURCE_CONFLICT", {
      confirm: "takeover blueprint blp-abc",
    });
    expect(protectedConfirmationFromError(takeover)).toBeNull();
  });

  it("asks for nothing when the refusal carries no phrase", () => {
    expect(
      protectedConfirmationFromError(codedGraphQLError(REFUSAL)),
    ).toBeNull();
    expect(
      protectedConfirmationFromError(
        codedGraphQLError(REFUSAL, { confirm: "" }),
      ),
    ).toBeNull();
  });

  it("ignores unrelated errors", () => {
    expect(protectedConfirmationFromError(new Error("boom"))).toBeNull();
  });
});

// w4/m126 added verbs whose names are more than one word ("take offline"), and
// w4/m127 added the datastore phrases, so the resource-name extraction cannot
// assume a single-token verb or the word "service".
describe("protectedServiceName", () => {
  it.each([
    ["sudo repoint service web", "web"],
    ["sudo take offline service web", "web"],
    ["sudo fail over database pg-1", "pg-1"],
    ["sudo delete key value kv-1", "kv-1"],
  ])("reads the resource name out of %s", (phrase, want) => {
    expect(protectedServiceName(phrase)).toBe(want);
  });

  it("falls back to the whole phrase when it does not parse", () => {
    expect(protectedServiceName("takeover blueprint blp-abc")).toBe(
      "takeover blueprint blp-abc",
    );
  });
});
