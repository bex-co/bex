import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseMutation = vi.fn();
const mockRefetchQueries = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
  useApolloClient: () => ({
    refetchQueries: (...args: unknown[]) => mockRefetchQueries(...args),
  }),
}));

// The project-detail page's membership table is fed by its route loader's
// snapshot of the singular `Project` query, which only a router.invalidate()
// re-runs — so the hook invalidates the router alongside refetching `Projects`.
const mockInvalidate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useRouter: () => ({
    invalidate: (...args: unknown[]) => mockInvalidate(...args),
  }),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

const mockUseProjects = vi.fn();
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => mockUseProjects(),
}));

import {
  ProjectsDocument,
  SetProjectServicesDocument,
  SetProjectDatabasesDocument,
  SetProjectKeyValuesDocument,
} from "@/graphql/definitions";
import { useMoveToProject } from "@/features/projects/hooks/use-move-to-project";

// One mutate spy per document so a test can assert which of the three
// full-replace mutations ran (and with which member list).
const setServices = vi.fn();
const setDatabases = vi.fn();
const setKeyValues = vi.fn();

function project(
  id: string,
  name: string,
  serviceIds: string[] = [],
  databaseIds: string[] = [],
  keyValueIds: string[] = [],
) {
  return { id, name, ownerId: "tea-1", serviceIds, databaseIds, keyValueIds };
}

beforeEach(() => {
  mockUseMutation.mockReset();
  mockRefetchQueries.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  mockUseProjects.mockReset();
  setServices.mockReset().mockResolvedValue({ data: {} });
  setDatabases.mockReset().mockResolvedValue({ data: {} });
  setKeyValues.mockReset().mockResolvedValue({ data: {} });
  mockRefetchQueries.mockResolvedValue([]);
  mockInvalidate.mockReset().mockResolvedValue(undefined);
  mockUseMutation.mockImplementation((doc: unknown) => {
    if (doc === SetProjectServicesDocument) return [setServices];
    if (doc === SetProjectDatabasesDocument) return [setDatabases];
    if (doc === SetProjectKeyValuesDocument) return [setKeyValues];
    throw new Error("unexpected mutation document");
  });
});

describe("useMoveToProject", () => {
  it("moves an ungrouped service into a project", async () => {
    mockUseProjects.mockReturnValue({
      projects: [project("prj-a", "Alpha", ["srv-existing"])],
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useMoveToProject("service"));

    let ok = false;
    await act(async () => {
      ok = await result.current.moveTo("srv-1", "web", "prj-a");
    });

    expect(ok).toBe(true);
    expect(setServices).toHaveBeenCalledTimes(1);
    expect(setServices).toHaveBeenCalledWith({
      variables: { id: "prj-a", serviceIds: ["srv-existing", "srv-1"] },
    });
    expect(toastSuccess).toHaveBeenCalledWith('"web" moved to "Alpha".');
    expect(toastError).not.toHaveBeenCalled();
    expect(mockRefetchQueries).toHaveBeenCalledWith({
      include: [ProjectsDocument],
    });
    expect(mockInvalidate).toHaveBeenCalledTimes(1);
  });

  it("moves a service between projects with two full-replace writes", async () => {
    mockUseProjects.mockReturnValue({
      projects: [
        project("prj-a", "Alpha", ["srv-1", "srv-2"]),
        project("prj-b", "Beta", ["srv-3"]),
      ],
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useMoveToProject("service"));

    let ok = false;
    await act(async () => {
      ok = await result.current.moveTo("srv-1", "web", "prj-b");
    });

    expect(ok).toBe(true);
    expect(setServices).toHaveBeenNthCalledWith(1, {
      variables: { id: "prj-a", serviceIds: ["srv-2"] },
    });
    expect(setServices).toHaveBeenNthCalledWith(2, {
      variables: { id: "prj-b", serviceIds: ["srv-3", "srv-1"] },
    });
    expect(toastSuccess).toHaveBeenCalledWith('"web" moved to "Beta".');
  });

  it("reports success even when the post-move refresh fails", async () => {
    // Regression (the "move seems not working" bug): selecting a menu item
    // closes the dropdown, unmounting the menu and aborting any refetch its
    // hook instance had in flight. That rejection must not be reported as a
    // failed move — the mutations already succeeded. Both refresh paths
    // reject here so an implementation awaiting either inside its error
    // handling fails this test.
    const aborted = new Error("The operation was aborted");
    mockRefetchQueries.mockRejectedValue(aborted);
    mockInvalidate.mockRejectedValue(aborted);
    mockUseProjects.mockReturnValue({
      projects: [project("prj-a", "Alpha")],
      refetch: vi.fn().mockRejectedValue(aborted),
    });
    const { result } = renderHook(() => useMoveToProject("service"));

    let ok = false;
    await act(async () => {
      ok = await result.current.moveTo("srv-1", "web", "prj-a");
    });

    expect(ok).toBe(true);
    expect(toastSuccess).toHaveBeenCalledWith('"web" moved to "Alpha".');
    expect(toastError).not.toHaveBeenCalled();
  });

  it("surfaces a failed move without refreshing", async () => {
    setServices.mockRejectedValue(new Error("boom"));
    mockUseProjects.mockReturnValue({
      projects: [project("prj-a", "Alpha")],
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useMoveToProject("service"));

    let ok = true;
    await act(async () => {
      ok = await result.current.moveTo("srv-1", "web", "prj-a");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith('Failed to move "web".');
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(mockRefetchQueries).not.toHaveBeenCalled();
    expect(mockInvalidate).not.toHaveBeenCalled();
  });

  it("is a no-op when the target already holds the resource", async () => {
    mockUseProjects.mockReturnValue({
      projects: [project("prj-a", "Alpha", ["srv-1"])],
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useMoveToProject("service"));

    let ok = true;
    await act(async () => {
      ok = await result.current.moveTo("srv-1", "web", "prj-a");
    });

    expect(ok).toBe(false);
    expect(setServices).not.toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastError).not.toHaveBeenCalled();
  });

  it("removes a service from its project", async () => {
    mockUseProjects.mockReturnValue({
      projects: [project("prj-a", "Alpha", ["srv-1", "srv-2"])],
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useMoveToProject("service"));

    let ok = false;
    await act(async () => {
      ok = await result.current.removeFromProject("srv-1", "web");
    });

    expect(ok).toBe(true);
    expect(setServices).toHaveBeenCalledWith({
      variables: { id: "prj-a", serviceIds: ["srv-2"] },
    });
    expect(toastSuccess).toHaveBeenCalledWith(
      '"web" removed from its project.',
    );
    expect(mockRefetchQueries).toHaveBeenCalledWith({
      include: [ProjectsDocument],
    });
    // Re-runs the project-detail route loader so its snapshot table drops the
    // removed service instead of leaving it stale until a manual reload.
    expect(mockInvalidate).toHaveBeenCalledTimes(1);
  });

  it("reports removal success even when the refresh fails", async () => {
    const aborted = new Error("The operation was aborted");
    mockRefetchQueries.mockRejectedValue(aborted);
    mockInvalidate.mockRejectedValue(aborted);
    mockUseProjects.mockReturnValue({
      projects: [project("prj-a", "Alpha", ["srv-1"])],
      refetch: vi.fn().mockRejectedValue(aborted),
    });
    const { result } = renderHook(() => useMoveToProject("service"));

    let ok = false;
    await act(async () => {
      ok = await result.current.removeFromProject("srv-1", "web");
    });

    expect(ok).toBe(true);
    expect(toastSuccess).toHaveBeenCalledWith(
      '"web" removed from its project.',
    );
    expect(toastError).not.toHaveBeenCalled();
  });

  it("moves a database through the databases mutation", async () => {
    mockUseProjects.mockReturnValue({
      projects: [project("prj-a", "Alpha", [], ["dpg-existing"])],
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useMoveToProject("database"));

    await act(async () => {
      await result.current.moveTo("dpg-1", "db", "prj-a");
    });

    expect(setDatabases).toHaveBeenCalledWith({
      variables: { id: "prj-a", databaseIds: ["dpg-existing", "dpg-1"] },
    });
    expect(setServices).not.toHaveBeenCalled();
  });

  it("resolves the current project of a resource", () => {
    mockUseProjects.mockReturnValue({
      projects: [
        project("prj-a", "Alpha", ["srv-1"]),
        project("prj-b", "Beta", ["srv-2"]),
      ],
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useMoveToProject("service"));

    expect(result.current.currentProjectId("srv-2")).toBe("prj-b");
    expect(result.current.currentProjectId("srv-9")).toBeNull();
  });
});
