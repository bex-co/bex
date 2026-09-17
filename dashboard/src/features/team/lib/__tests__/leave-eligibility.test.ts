import { describe, expect, it } from "vitest";
import { leaveEligibility } from "@/features/team/lib/leave-eligibility";
import type { MemberView } from "@/features/team/types";

function member(overrides: Partial<MemberView> = {}): MemberView {
  return {
    subject: "subj",
    userId: "own-1",
    email: "a@example.com",
    role: "DEVELOPER",
    createdAt: null,
    mfaEnabled: false,
    identityResolved: true,
    isOwner: false,
    isSelf: false,
    ...overrides,
  };
}

// The UI pre-check must agree with the server's refusals (w5/m102): the owner
// and the last admin cannot leave, everyone else can.
describe("leaveEligibility", () => {
  it("lets an ordinary member leave", () => {
    expect(
      leaveEligibility([
        member({ subject: "admin", role: "ADMIN" }),
        member({ subject: "me", isSelf: true }),
      ]),
    ).toEqual({ isMember: true, blockedReason: null });
  });

  it("refuses the workspace owner", () => {
    expect(
      leaveEligibility([
        member({ subject: "me", isSelf: true, isOwner: true, role: "ADMIN" }),
        member({ subject: "other", role: "ADMIN" }),
      ]),
    ).toEqual({ isMember: true, blockedReason: "team.leaveOwnerReason" });
  });

  it("refuses the last admin", () => {
    expect(
      leaveEligibility([
        member({ subject: "me", isSelf: true, role: "ADMIN" }),
        member({ subject: "dev", role: "DEVELOPER" }),
      ]),
    ).toEqual({ isMember: true, blockedReason: "team.leaveLastAdminReason" });
  });

  it("lets an admin leave once a second admin exists", () => {
    expect(
      leaveEligibility([
        member({ subject: "me", isSelf: true, role: "ADMIN" }),
        member({ subject: "other", role: "ADMIN" }),
      ]).blockedReason,
    ).toBeNull();
  });

  it("reports no membership when the caller is not in the list", () => {
    // A non-member has nothing to leave — distinct from being refused, and the
    // card renders nothing rather than a disabled control.
    expect(leaveEligibility([member({ subject: "someone-else" })])).toEqual({
      isMember: false,
      blockedReason: null,
    });
    expect(leaveEligibility([])).toEqual({
      isMember: false,
      blockedReason: null,
    });
  });
});
