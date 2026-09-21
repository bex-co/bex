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

  it("extracts the blueprint takeover phrase (w8/m23)", () => {
    const err = new Error(
      'service "web" is managed by blueprint blp-abc; retry with confirm="takeover blueprint blp-abc" to transfer ownership to this blueprint',
    );
    expect(protectedConfirmationFromError(err)).toBe(
      "takeover blueprint blp-abc",
    );
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
