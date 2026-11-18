import { afterEach, describe, expect, it, vi } from "vitest";
import Cookies from "js-cookie";
import {
  clearPersistedWorkspaceId,
  getPersistedWorkspaceShape,
  persistWorkspaceId,
  persistWorkspaceShape,
  workspaceShape,
  WORKSPACE_SELECTION_KEY,
  WORKSPACE_SHAPE_KEY,
  type WorkspaceShape,
} from "@/features/workspaces/lib/selection";
import type { WorkspaceView } from "@/features/workspaces/types";

const standalone: WorkspaceView = {
  id: "tea-solo",
  name: "solo",
  slug: "solo",
  plan: "hobby",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: null,
  primary: false,
  privacy: null,
};

const organization = {
  id: "org-1",
  name: "Acme & Söhne",
  primaryWorkspaceId: "tea-hq",
  requireTwoFactor: true,
  requireGoogleLogin: false,
};

const secondary: WorkspaceView = {
  ...standalone,
  id: "tea-staging",
  plan: "scale",
  organization,
  privacy: "invite_only",
};

const orgShape: WorkspaceShape = {
  role: "admin",
  plan: "scale",
  organization,
  planOwner: { primaryWorkspaceId: "tea-hq" },
  otherWorkspaces: true,
};

afterEach(() => {
  Cookies.remove(WORKSPACE_SELECTION_KEY, { path: "/" });
  Cookies.remove(WORKSPACE_SHAPE_KEY, { path: "/" });
});

describe("the selected workspace's shape (w3/073)", () => {
  it("is what the pending frame reads from the list", () => {
    expect(workspaceShape(standalone, [standalone])).toEqual({
      role: "admin",
      plan: "hobby",
      organization: null,
      planOwner: null,
      otherWorkspaces: false,
    });
    const hq = { ...secondary, id: "tea-hq", primary: true };
    expect(workspaceShape(secondary, [secondary, hq])).toEqual(orgShape);
    // Without the primary in the list there is no page of it to link to.
    expect(
      workspaceShape(secondary, [secondary, standalone]).planOwner,
    ).toEqual({ primaryWorkspaceId: null });
  });

  it("reads back for the workspace it was stored for, and no other", () => {
    persistWorkspaceShape("tea-staging", orgShape);

    expect(getPersistedWorkspaceShape("tea-staging")).toEqual(orgShape);
    expect(getPersistedWorkspaceShape("tea-hq")).toBeNull();
    expect(getPersistedWorkspaceShape(null)).toBeNull();
    expect(getPersistedWorkspaceShape(undefined)).toBeNull();
  });

  it("holds no workspace name, the copy a pending frame would print", () => {
    persistWorkspaceShape(
      "tea-staging",
      workspaceShape({ ...secondary, name: "secret-project" }, [secondary]),
    );

    expect(Cookies.get(WORKSPACE_SHAPE_KEY)).not.toContain("secret-project");
  });

  it("is not rewritten while unchanged", () => {
    persistWorkspaceShape("tea-staging", orgShape);
    const set = vi.spyOn(Cookies, "set");

    persistWorkspaceShape("tea-staging", { ...orgShape });
    expect(set).not.toHaveBeenCalled();

    persistWorkspaceShape("tea-staging", { ...orgShape, plan: "pro" });
    expect(set).toHaveBeenCalledOnce();
    expect(getPersistedWorkspaceShape("tea-staging")?.plan).toBe("pro");
    set.mockRestore();
  });

  it.each([
    ["not JSON", "{"],
    ["another version", JSON.stringify({ ...orgShape, v: 2, id: "tea-x" })],
    ["no id", JSON.stringify({ ...orgShape, v: 1 })],
    [
      "a non-boolean workspace count",
      JSON.stringify({ ...orgShape, v: 1, id: "tea-x", otherWorkspaces: 2 }),
    ],
    [
      "an organization without its fields",
      JSON.stringify({ ...orgShape, v: 1, id: "tea-x", organization: {} }),
    ],
    [
      "a malformed plan owner",
      JSON.stringify({ ...orgShape, v: 1, id: "tea-x", planOwner: 7 }),
    ],
    ["an array", JSON.stringify([1, "tea-x"])],
  ])("is ignored when the cookie holds %s", (_, value) => {
    Cookies.set(WORKSPACE_SHAPE_KEY, value, { path: "/" });

    expect(getPersistedWorkspaceShape("tea-x")).toBeNull();
  });

  it("is cleared with the selection after the last workspace goes", () => {
    persistWorkspaceId("tea-staging");
    persistWorkspaceShape("tea-staging", orgShape);

    clearPersistedWorkspaceId();

    expect(Cookies.get(WORKSPACE_SELECTION_KEY)).toBeUndefined();
    expect(getPersistedWorkspaceShape("tea-staging")).toBeNull();
  });
});
