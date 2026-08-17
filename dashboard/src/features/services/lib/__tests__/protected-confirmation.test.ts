import { describe, it, expect } from "vitest";
import { protectedConfirmationFromError } from "../protected-confirmation";

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
