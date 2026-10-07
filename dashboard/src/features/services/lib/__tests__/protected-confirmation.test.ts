import { describe, it, expect } from "vitest";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import {
  protectedConfirmationFromError,
  protectedRefusalFromError,
  withProtectedRetry,
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

// The dialog names the resource from the refusal's own `name` param (w5/m130),
// so a multi-word verb ("take offline") or a datastore phrase needs no parsing,
// and a phrase the server rewords cannot change the name shown.
describe("protectedRefusalFromError", () => {
  it.each([
    ["sudo take offline service web", "web"],
    ["sudo fail over database pg-1", "pg-1"],
    ["the server may word the phrase however it likes", "kv-1"],
  ])(
    "reads the name the refusal carries, whatever the phrase (%s)",
    (confirm, name) => {
      expect(
        protectedRefusalFromError(
          codedGraphQLError(REFUSAL, { confirm, name }),
        ),
      ).toEqual({ confirm, name });
    },
  );

  it("names the phrase when the refusal carries no name", () => {
    expect(
      protectedRefusalFromError(
        codedGraphQLError(REFUSAL, { confirm: "sudo delete service web" }),
      ),
    ).toEqual({
      confirm: "sudo delete service web",
      name: "sudo delete service web",
    });
  });

  it("finds nothing without the code", () => {
    expect(
      protectedRefusalFromError(
        uncodedGraphQLError('"web" is a member of a protected environment'),
      ),
    ).toBeNull();
  });
});

describe("withProtectedRetry", () => {
  it("asks with the phrase and the refusal's name, then retries with the confirmation", async () => {
    const asked: Array<[string, string | undefined]> = [];
    const attempts: Array<string | undefined> = [];
    const result = await withProtectedRetry(
      async (phrase, name) => {
        asked.push([phrase, name]);
        return phrase;
      },
      async (confirm) => {
        attempts.push(confirm);
        if (!confirm) {
          throw codedGraphQLError(REFUSAL, {
            confirm: "sudo take offline service web",
            verb: "take offline",
            name: "web",
          });
        }
        return "done";
      },
    );
    expect(result).toBe("done");
    expect(asked).toEqual([["sudo take offline service web", "web"]]);
    expect(attempts).toEqual([undefined, "sudo take offline service web"]);
  });
});
