import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import {
  BuildSourceBuildsDocument,
  BuildSourceDocument,
  BuildSourcesDocument,
  ServicesDocument,
} from "@/graphql/definitions";

const mockMutate = vi.fn();
const mockRefetchQueries = vi.fn(async () => []);
const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
const workspaceState = vi.hoisted(() => ({ id: "tea-1" as string | null }));

vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mockMutate],
  useApolloClient: () => ({ refetchQueries: mockRefetchQueries }),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: workspaceState.id }),
}));
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => mockToastSuccess(...args),
    error: (...args: unknown[]) => mockToastError(...args),
  },
}));

import { useCreateBuildSource } from "../use-create-build-source";
import { useDeleteBuildSource } from "../use-delete-build-source";
import { useTriggerBuild } from "../use-trigger-build";
import { useUpdateBuildSource } from "../use-update-build-source";

function gqlError(message: string, extensions: Record<string, unknown>) {
  return new CombinedGraphQLErrors({
    errors: [{ message, extensions }],
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  workspaceState.id = "tea-1";
});

describe("useCreateBuildSource", () => {
  it("creates a build source and refetches BuildSources (and Services when linked)", async () => {
    mockMutate.mockResolvedValue({
      data: {
        createBuildSource: {
          id: "bds-1",
          name: "shared-api",
          ownerId: "tea-1",
          projectId: null,
          git: { repoUrl: "https://github.com/acme/api", branch: "main", runtime: "go" },
          image: null,
          serviceLinks: [{ id: "srv-1", name: "api" }],
          createdAt: "2026-11-24T00:00:00Z",
          updatedAt: "2026-11-24T00:00:00Z",
        },
      },
    });

    const { result } = renderHook(() => useCreateBuildSource());
    let created: unknown = null;
    await act(async () => {
      created = await result.current.create({
        name: "shared-api",
        git: { repoUrl: "https://github.com/acme/api", runtime: "go" },
        serviceIds: ["srv-1"],
      });
    });

    expect(created).toMatchObject({ id: "bds-1", name: "shared-api" });
    expect(mockMutate).toHaveBeenCalledWith({
      variables: {
        ownerId: "tea-1",
        name: "shared-api",
        git: { repoUrl: "https://github.com/acme/api", runtime: "go" },
        serviceIds: ["srv-1"],
      },
    });
    expect(mockRefetchQueries).toHaveBeenCalledWith({
      include: [BuildSourcesDocument, ServicesDocument],
    });
    expect(mockToastSuccess).toHaveBeenCalled();
  });

  it("sets inline nameTaken on BUILD_SOURCE_NAME_TAKEN without firing an error toast", async () => {
    mockMutate.mockRejectedValue(
      gqlError('a build source named "shared-api" already exists', {
        code: "BUILD_SOURCE_NAME_TAKEN",
      }),
    );

    const { result } = renderHook(() => useCreateBuildSource());
    await act(async () => {
      const created = await result.current.create({
        name: "shared-api",
        image: { imageUrl: "nginx:1.27" },
      });
      expect(created).toBeNull();
    });

    expect(result.current.nameTaken).toBe(true);
    expect(mockToastError).not.toHaveBeenCalled();
    act(() => result.current.clearNameTaken());
    expect(result.current.nameTaken).toBe(false);
  });

  it("extracts BUILD_SOURCE_PARTIALLY_LINKED details and refreshes caches", async () => {
    mockMutate.mockRejectedValue(
      gqlError(
        "build source bds-partial was created and linked to srv-1; linking service srv-2 failed",
        {
          code: "BUILD_SOURCE_PARTIALLY_LINKED",
          buildSourceId: "bds-partial",
          linkedServiceIds: ["srv-1"],
          failedServiceId: "srv-2",
        },
      ),
    );

    const { result } = renderHook(() => useCreateBuildSource());
    await act(async () => {
      const created = await result.current.create({
        name: "shared-api",
        image: { imageUrl: "nginx:1.27" },
        serviceIds: ["srv-1", "srv-2"],
      });
      expect(created).toBeNull();
    });

    expect(result.current.partiallyLinked).toEqual({
      buildSourceId: "bds-partial",
      linkedServiceIds: ["srv-1"],
      failedServiceId: "srv-2",
      message:
        "Build source bds-partial was created and linked to srv-1; linking service srv-2 failed",
    });
    expect(mockRefetchQueries).toHaveBeenCalledWith({
      include: [BuildSourcesDocument, ServicesDocument],
    });
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("fails gracefully when no workspace is selected", async () => {
    workspaceState.id = null;
    const { result } = renderHook(() => useCreateBuildSource());
    await act(async () => {
      expect(
        await result.current.create({
          name: "x",
          image: { imageUrl: "nginx:latest" },
        }),
      ).toBeNull();
    });
    expect(mockMutate).not.toHaveBeenCalled();
    expect(mockToastError).toHaveBeenCalled();
  });
});

describe("useDeleteBuildSource", () => {
  it("evicts the deleted BuildSource from Apollo cache and refetches lists", async () => {
    const evict = vi.fn();
    const gc = vi.fn();
    const identify = vi.fn(() => "BuildSource:bds-1");
    mockMutate.mockImplementation(
      async ({ update }: { update?: (cache: unknown) => void }) => {
        update?.({ identify, evict, gc });
        return { data: { deleteBuildSource: true } };
      },
    );

    const { result } = renderHook(() => useDeleteBuildSource());
    await act(async () => {
      expect(await result.current.remove("bds-1", "shared-api")).toBe(true);
    });

    expect(identify).toHaveBeenCalledWith({
      __typename: "BuildSource",
      id: "bds-1",
    });
    expect(evict).toHaveBeenCalledWith({ id: "BuildSource:bds-1" });
    expect(gc).toHaveBeenCalled();
    expect(mockRefetchQueries).toHaveBeenCalledWith({
      include: [BuildSourcesDocument],
    });
  });
});

describe("useUpdateBuildSource & useTriggerBuild", () => {
  it("refetches BuildSources and BuildSource after update", async () => {
    mockMutate.mockResolvedValue({
      data: {
        updateBuildSource: {
          id: "bds-1",
          name: "renamed",
          ownerId: "tea-1",
          projectId: null,
          git: null,
          image: { imageUrl: "nginx:1.27" },
          serviceLinks: [],
          createdAt: "2026-11-24T00:00:00Z",
          updatedAt: "2026-11-24T00:01:00Z",
        },
      },
    });
    const { result } = renderHook(() => useUpdateBuildSource());
    await act(async () => {
      const updated = await result.current.update({
        id: "bds-1",
        name: "renamed",
      });
      expect(updated?.name).toBe("renamed");
    });
    expect(mockRefetchQueries).toHaveBeenCalledWith({
      include: [BuildSourcesDocument, BuildSourceDocument],
    });
  });

  it("refetches BuildSourceBuilds after triggerBuild", async () => {
    mockMutate.mockResolvedValue({
      data: {
        triggerBuild: {
          id: "bld-9",
          buildSourceId: "bds-1",
          cursor: "dep-9",
          createdAt: "2026-11-24T00:05:00Z",
          updatedAt: "2026-11-24T00:05:00Z",
          buildRun: { id: "dep-9", status: "created" },
          image: null,
          deploys: [],
          skippedServices: [],
        },
      },
    });
    const { result } = renderHook(() => useTriggerBuild());
    await act(async () => {
      const build = await result.current.trigger({ buildSourceId: "bds-1" });
      expect(build?.id).toBe("bld-9");
    });
    expect(mockRefetchQueries).toHaveBeenCalledWith({
      include: [BuildSourceBuildsDocument],
    });
  });
});
