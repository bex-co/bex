import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  clearWorkspaceLoginBlocks,
  isLoginRequirementExemptPath,
  reportWorkspaceLoginRequired,
  subscribeWorkspaceLoginBlocks,
  workspaceLoginBlock,
} from "../workspace-login-block";
import { useWorkspaceLoginBlock } from "../../hooks/use-workspace-login-block";

beforeEach(() => {
  clearWorkspaceLoginBlocks();
});

describe("workspace login block record (w22/m402, w7/m465)", () => {
  it("records a workspace once however many queries report it", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeWorkspaceLoginBlocks(listener);

    reportWorkspaceLoginRequired("tea-a", "twoFactor");
    reportWorkspaceLoginRequired("tea-a", "twoFactor");
    reportWorkspaceLoginRequired("tea-a", "twoFactor");

    expect(workspaceLoginBlock("tea-a")).toBe("twoFactor");
    // One notification, however many reports.
    expect(listener).toHaveBeenCalledTimes(1);
    unsubscribe();
  });

  // A member who fails both is refused for Google first; once they sign in
  // with Google, the server names the second factor, and the page follows.
  it("follows the latest requirement the server names for a workspace", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeWorkspaceLoginBlocks(listener);

    reportWorkspaceLoginRequired("tea-a", "googleLogin");
    expect(workspaceLoginBlock("tea-a")).toBe("googleLogin");
    reportWorkspaceLoginRequired("tea-a", "twoFactor");
    expect(workspaceLoginBlock("tea-a")).toBe("twoFactor");

    expect(listener).toHaveBeenCalledTimes(2);
    unsubscribe();
  });

  it("keeps workspaces apart, and clears all of them together", () => {
    reportWorkspaceLoginRequired("tea-a", "twoFactor");
    reportWorkspaceLoginRequired("tea-b", "googleLogin");
    expect(workspaceLoginBlock("tea-a")).toBe("twoFactor");
    expect(workspaceLoginBlock("tea-b")).toBe("googleLogin");
    expect(workspaceLoginBlock("tea-c")).toBeNull();

    clearWorkspaceLoginBlocks();
    expect(workspaceLoginBlock("tea-a")).toBeNull();
    expect(workspaceLoginBlock("tea-b")).toBeNull();
  });

  it("does not notify for a clear that changes nothing, or after unsubscribing", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeWorkspaceLoginBlocks(listener);
    clearWorkspaceLoginBlocks();
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
      return String(useWorkspaceLoginBlock("tea-a"));
    }
    expect(renderToString(createElement(Probe))).toBe("null");
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
