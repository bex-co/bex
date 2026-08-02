import { workspaceStatusLabel } from "../workspace-status";

const translate = {
  role: (slug: string) => `role:${slug}`,
  plan: (slug: string) => `plan:${slug}`,
};

describe("workspaceStatusLabel", () => {
  it("localizes known role and plan enums in role · plan order", () => {
    expect(
      workspaceStatusLabel({ role: "admin", plan: "hobby" }, translate),
    ).toBe("role:admin · plan:hobby");
  });

  it("title-cases unknown values instead of emitting a missing-key string", () => {
    expect(
      workspaceStatusLabel({ role: "SUPERUSER", plan: "galaxy" }, translate),
    ).toBe("Superuser · Galaxy");
  });

  it("omits a missing field rather than showing it blank", () => {
    expect(
      workspaceStatusLabel({ role: "viewer", plan: null }, translate),
    ).toBe("role:viewer");
    expect(workspaceStatusLabel({ role: null, plan: "pro" }, translate)).toBe(
      "plan:pro",
    );
  });

  it("returns an empty status for an all-missing workspace", () => {
    expect(workspaceStatusLabel({ role: null, plan: null }, translate)).toBe(
      "",
    );
    expect(workspaceStatusLabel({ role: "  ", plan: "" }, translate)).toBe("");
  });
});
