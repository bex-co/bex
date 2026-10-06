import { describe, it, expect } from "vitest";
import {
  protectedConfirmationFromError,
  protectedServiceName,
} from "../protected-confirmation";

describe("protectedConfirmationFromError", () => {
  it("extracts the protected-environment phrase", () => {
    const err = new Error(
      '"api" is a member of a protected environment; retry with confirm="sudo deploy service api" to deploy it',
    );
    expect(protectedConfirmationFromError(err)).toBe("sudo deploy service api");
  });

  // Blueprint takeovers share the confirm-phrase convention but carry a code,
  // so blueprintTakeoverFromError classifies them first; this helper answers
  // only the protected-environment refusal (w5/m125).
  it("leaves Blueprint takeover refusals to their own classifier", () => {
    for (const message of [
      'service "web" is managed by blueprint blp-abc; retry with confirm="takeover blueprint blp-abc" to transfer ownership to this blueprint',
      'blueprint blp-1 ("bpA") already tracks https://github.com/o/r@main from "a.yaml"; update it with updateBlueprint to change its path, or retry with confirm="takeover blueprint blp-1" to replace it',
    ]) {
      expect(protectedConfirmationFromError(new Error(message))).toBeNull();
    }
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
