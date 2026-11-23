import { describe, expect, it } from "vitest";
import {
  adminOnlyErrorMessage,
  isAdminOnlyRefusal,
} from "@/features/capabilities/lib/admin-only";
import { codedGraphQLError } from "@/test/mocks/apollo";

const t = (key: string) =>
  key === "capabilities.actionAdminOnly"
    ? "This action is limited to admins."
    : key;

// w4/m594: a raced click on a control a protected environment keeps to
// Admins reads the reason the disabled control shows, whatever the server's
// words.
describe("adminOnlyErrorMessage", () => {
  it("reads bex-api's coded refusal as Render's admin-only reason", () => {
    const err = codedGraphQLError("PROTECTED_ENVIRONMENT_ADMIN_REQUIRED", {
      environmentId: "evm-1",
    });
    expect(isAdminOnlyRefusal(err)).toBe(true);
    expect(adminOnlyErrorMessage(err, "Couldn't save", t)).toBe(
      "This action is limited to admins.",
    );
  });

  it("keeps any other refusal's words", () => {
    const err = codedGraphQLError("FORBIDDEN", {}, "forbidden");
    expect(isAdminOnlyRefusal(err)).toBe(false);
    expect(adminOnlyErrorMessage(err, "Couldn't save", t)).not.toBe(
      "This action is limited to admins.",
    );
    expect(isAdminOnlyRefusal(new Error("network down"))).toBe(false);
  });
});
