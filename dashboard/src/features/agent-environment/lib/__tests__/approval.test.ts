import { describe, expect, it } from "vitest";
import {
  APPROVAL_MODES,
  parseApprovalMode,
  sessionApprovalInput,
  sessionModeAllowed,
} from "@/features/agent-environment/lib/approval";

describe("approval mode rules (ADR100 D3)", () => {
  it("reads unknown wire values as the autonomous default", () => {
    expect(parseApprovalMode("risky")).toBe("risky");
    expect(parseApprovalMode(null)).toBe("autonomous");
    expect(parseApprovalMode("paranoid")).toBe("autonomous");
  });

  it("always allows raising the mode, and lowering only when bypass is allowed", () => {
    // Truth table over every (session, workspace) pair.
    for (const workspace of APPROVAL_MODES) {
      for (const mode of APPROVAL_MODES) {
        const raising =
          APPROVAL_MODES.indexOf(mode) >= APPROVAL_MODES.indexOf(workspace);
        expect(sessionModeAllowed(mode, workspace, false)).toBe(raising);
        expect(sessionModeAllowed(mode, workspace, true)).toBe(true);
      }
    }
  });

  it("sends nothing for the workspace's own mode, bypass for autonomous, else the mode", () => {
    expect(sessionApprovalInput(null, "risky")).toEqual({});
    expect(sessionApprovalInput("risky", "risky")).toEqual({});
    expect(sessionApprovalInput("autonomous", "risky")).toEqual({
      bypassApproval: true,
    });
    expect(sessionApprovalInput("all", "risky")).toEqual({
      approvalMode: "all",
    });
    expect(sessionApprovalInput("risky", "autonomous")).toEqual({
      approvalMode: "risky",
    });
  });
});
