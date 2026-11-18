import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import {
  clearLoginBlocks,
  isLoginRequirementExemptPath,
  organizationLoginBlock,
  reportLoginRefusal,
  subscribeLoginBlocks,
  workspaceLoginBlock,
} from "../workspace-login-block";
import {
  useOrganizationLoginBlock,
  useWorkspaceLoginBlock,
} from "../../hooks/use-workspace-login-block";

/** A workspace's own setting refused it. */
function reportWorkspaceLoginRequired(
  workspaceId: string,
  requirement: WorkspaceLoginRequirement,
) {
  reportLoginRefusal({ workspaceId, organization: null, requirement });
}

const ACME = { id: "org-1", name: "Acme" };

beforeEach(() => {
  clearLoginBlocks();
});

describe("workspace login block record (w22/m402, w7/m465)", () => {
  it("records a workspace once however many queries report it", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeLoginBlocks(listener);

    reportWorkspaceLoginRequired("tea-a", "twoFactor");
    reportWorkspaceLoginRequired("tea-a", "twoFactor");
    reportWorkspaceLoginRequired("tea-a", "twoFactor");

    expect(workspaceLoginBlock("tea-a")).toEqual({
      requirement: "twoFactor",
      organization: null,
    });
    // One notification, however many reports.
    expect(listener).toHaveBeenCalledTimes(1);
    unsubscribe();
  });

  // A member who fails both is refused for Google first; once they sign in
  // with Google, the server names the second factor, and the page follows.
  it("follows the latest requirement the server names for a workspace", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeLoginBlocks(listener);

    reportWorkspaceLoginRequired("tea-a", "googleLogin");
    expect(workspaceLoginBlock("tea-a")).toEqual({
      requirement: "googleLogin",
      organization: null,
    });
    reportWorkspaceLoginRequired("tea-a", "twoFactor");
    expect(workspaceLoginBlock("tea-a")).toEqual({
      requirement: "twoFactor",
      organization: null,
    });

    expect(listener).toHaveBeenCalledTimes(2);
    unsubscribe();
  });

  it("keeps workspaces apart, and clears all of them together", () => {
    reportWorkspaceLoginRequired("tea-a", "twoFactor");
    reportWorkspaceLoginRequired("tea-b", "googleLogin");
    expect(workspaceLoginBlock("tea-a")).toEqual({
      requirement: "twoFactor",
      organization: null,
    });
    expect(workspaceLoginBlock("tea-b")).toEqual({
      requirement: "googleLogin",
      organization: null,
    });
    expect(workspaceLoginBlock("tea-c")).toBeNull();

    clearLoginBlocks();
    expect(workspaceLoginBlock("tea-a")).toBeNull();
    expect(workspaceLoginBlock("tea-b")).toBeNull();
  });

  it("does not notify for a clear that changes nothing, or after unsubscribing", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeLoginBlocks(listener);
    clearLoginBlocks();
    expect(listener).not.toHaveBeenCalled();

    unsubscribe();
    reportWorkspaceLoginRequired("tea-a", "googleLogin");
    expect(listener).not.toHaveBeenCalled();
  });

  // The record is written only by the browser's client, so a server render
  // (which must agree with the first client render) always sees it empty.
  it("reads empty on the server whatever the browser holds", () => {
    reportWorkspaceLoginRequired("tea-a", "googleLogin");
    function Probe() {
      return `${String(useWorkspaceLoginBlock("tea-a"))} ${String(useOrganizationLoginBlock("org-1"))}`;
    }
    reportLoginRefusal({
      workspaceId: null,
      organization: ACME,
      requirement: "twoFactor",
    });
    expect(renderToString(createElement(Probe))).toBe("null null");
  });
});

// w7/m466: an org's own setting refuses with the same codes, naming the org.
describe("an organization's login block (w7/m466)", () => {
  it("records a refusal in an org workspace against the workspace and the org", () => {
    reportLoginRefusal({
      workspaceId: "tea-a",
      organization: ACME,
      requirement: "googleLogin",
    });
    const block = { requirement: "googleLogin", organization: ACME };
    expect(workspaceLoginBlock("tea-a")).toEqual(block);
    expect(organizationLoginBlock("org-1")).toEqual(block);
    // Another org, and the same id as a workspace, stay unblocked.
    expect(organizationLoginBlock("org-2")).toBeNull();
    expect(workspaceLoginBlock("org-1")).toBeNull();
  });

  it("records a refusal on the org's own objects against the org alone", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeLoginBlocks(listener);
    for (let i = 0; i < 3; i++) {
      reportLoginRefusal({
        workspaceId: null,
        organization: ACME,
        requirement: "twoFactor",
      });
    }
    expect(organizationLoginBlock("org-1")).toEqual({
      requirement: "twoFactor",
      organization: ACME,
    });
    expect(listener).toHaveBeenCalledTimes(1);

    clearLoginBlocks();
    expect(organizationLoginBlock("org-1")).toBeNull();
    expect(listener).toHaveBeenCalledTimes(2);
    unsubscribe();
  });

  // A workspace refused by its own setting does not block its org's pages.
  it("leaves the org unblocked when the workspace's own setting refused", () => {
    reportWorkspaceLoginRequired("tea-a", "twoFactor");
    expect(organizationLoginBlock("org-1")).toBeNull();
  });
});

describe("isLoginRequirementExemptPath (w22/m402)", () => {
  it.each(["/settings", "/settings/", "/new/workspace", "/new/workspace/"])(
    "leaves %s open to a blocked member",
    (path) => expect(isLoginRequirementExemptPath(path)).toBe(true),
  );

  // Every workspace-scoped page stays walled, including look-alikes.
  it.each([
    "/",
    "/workspace/settings",
    "/billing",
    "/services/srv-1",
    "/settingsx",
    "/new/database",
    "/new",
  ])("walls %s", (path) =>
    expect(isLoginRequirementExemptPath(path)).toBe(false),
  );
});
