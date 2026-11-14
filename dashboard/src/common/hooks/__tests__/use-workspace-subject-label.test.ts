import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import {
  SUBJECT_LABEL_PENDING,
  useWorkspaceSubjectLabel,
} from "../use-workspace-subject-label";

const query = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => query(...args),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

const roster = {
  workspaceMembers: [
    { subject: "usr-ada", userId: "usr-ada", email: "ada@example.com" },
  ],
};

describe("useWorkspaceSubjectLabel (w4/602)", () => {
  beforeEach(() => query.mockReset());

  it("never shows the raw id while the roster loads", () => {
    query.mockReturnValue({ data: undefined, loading: true });
    const { result } = renderHook(() => useWorkspaceSubjectLabel("usr-ada"));
    expect(result.current).toBe(SUBJECT_LABEL_PENDING);
  });

  it("names a member by email once the roster settles", () => {
    query.mockReturnValue({ data: roster, loading: false });
    const { result } = renderHook(() => useWorkspaceSubjectLabel("usr-ada"));
    expect(result.current).toBe("ada@example.com");
  });

  it("shows a removed member's id after the roster settles without a match", () => {
    query.mockReturnValue({ data: roster, loading: false });
    const { result } = renderHook(() => useWorkspaceSubjectLabel("usr-gone"));
    expect(result.current).toBe("usr-gone");
  });

  it("uses a label the caller already holds from first paint, without the roster", () => {
    query.mockReturnValue({ data: undefined, loading: false });
    const { result } = renderHook(() =>
      useWorkspaceSubjectLabel("usr-ada", "ada@example.com"),
    );
    expect(result.current).toBe("ada@example.com");
    expect(query.mock.calls[0][1]).toMatchObject({ skip: true });
  });
});
