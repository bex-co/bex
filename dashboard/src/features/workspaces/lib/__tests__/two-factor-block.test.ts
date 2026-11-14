import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  clearWorkspaceTwoFactorBlocks,
  isTwoFactorExemptPath,
  isWorkspaceTwoFactorBlocked,
  reportWorkspaceTwoFactorRequired,
  subscribeWorkspaceTwoFactorBlocks,
} from "../two-factor-block";
import { useWorkspaceTwoFactorBlocked } from "../../hooks/use-workspace-two-factor-blocked";

beforeEach(() => {
  clearWorkspaceTwoFactorBlocks();
});

describe("workspace two-factor block record (w22/m402)", () => {
  it("records a workspace once however many queries report it", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeWorkspaceTwoFactorBlocks(listener);

    reportWorkspaceTwoFactorRequired("tea-a");
    reportWorkspaceTwoFactorRequired("tea-a");
    reportWorkspaceTwoFactorRequired("tea-a");

    expect(isWorkspaceTwoFactorBlocked("tea-a")).toBe(true);
    // One notification, however many reports.
    expect(listener).toHaveBeenCalledTimes(1);
    unsubscribe();
  });

  it("keeps workspaces apart, and clears all of them together", () => {
    reportWorkspaceTwoFactorRequired("tea-a");
    reportWorkspaceTwoFactorRequired("tea-b");
    expect(isWorkspaceTwoFactorBlocked("tea-a")).toBe(true);
    expect(isWorkspaceTwoFactorBlocked("tea-b")).toBe(true);

    clearWorkspaceTwoFactorBlocks();
    expect(isWorkspaceTwoFactorBlocked("tea-a")).toBe(false);
    expect(isWorkspaceTwoFactorBlocked("tea-b")).toBe(false);
  });

  it("does not notify for a clear that changes nothing, or after unsubscribing", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeWorkspaceTwoFactorBlocks(listener);
    clearWorkspaceTwoFactorBlocks();
    expect(listener).not.toHaveBeenCalled();

    unsubscribe();
    reportWorkspaceTwoFactorRequired("tea-a");
    expect(listener).not.toHaveBeenCalled();
  });

  // The record is written only by the browser's client, so a server render
  // (which must agree with the first client render) always sees it empty.
  it("reads empty on the server whatever the browser holds", () => {
    reportWorkspaceTwoFactorRequired("tea-a");
    function Probe() {
      return String(useWorkspaceTwoFactorBlocked("tea-a"));
    }
    expect(renderToString(createElement(Probe))).toBe("false");
  });
});

describe("isTwoFactorExemptPath (w22/m402)", () => {
  it.each(["/settings", "/settings/", "/new/workspace", "/new/workspace/"])(
    "leaves %s open to a blocked member",
    (path) => expect(isTwoFactorExemptPath(path)).toBe(true),
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
  ])("walls %s", (path) => expect(isTwoFactorExemptPath(path)).toBe(false));
});
