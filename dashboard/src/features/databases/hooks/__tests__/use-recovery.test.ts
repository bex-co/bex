import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { Kind, type DocumentNode, type SelectionSetNode } from "graphql";
import { useRecovery } from "@/features/databases/hooks/use-recovery";
import {
  DatabaseExportLinksDocument,
  DatabaseExportsDocument,
} from "@/graphql/definitions";

const useQuery = vi.fn();
const useMutation = vi.fn();
const clientQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => useQuery(...args),
  useMutation: (...args: unknown[]) => useMutation(...args),
  useApolloClient: () => ({ query: clientQuery }),
}));

const openDownloadURL = vi.fn();
vi.mock("@/common/lib/download-file", () => ({
  openDownloadURL: (...args: unknown[]) => openDownloadURL(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

const createExportMutation = vi.fn();
const recoverMutation = vi.fn();
const stopPolling = vi.fn();

beforeEach(() => {
  vi.clearAllMocks();
  useQuery.mockReturnValue({
    data: undefined,
    loading: false,
    refetch: vi.fn(async () => undefined),
    startPolling: vi.fn(),
    stopPolling,
  });
  useMutation
    .mockReturnValueOnce([createExportMutation, { loading: false }])
    .mockReturnValueOnce([recoverMutation, { loading: false }]);
});

describe("useRecovery post-create identity", () => {
  it("does not poll exports while recovery information is unreadable", () => {
    const infoError = new Error("forbidden");
    const startExportPolling = vi.fn();
    const stopExportPolling = vi.fn();
    useQuery
      .mockReturnValueOnce({
        data: {
          databaseRecoveryInfo: {
            enabled: true,
            earliestRecoveryTime: "2026-07-13T12:00:00Z",
            latestRecoveryTime: "2026-07-14T12:00:00Z",
            backups: [],
          },
        },
        error: infoError,
        loading: false,
      })
      .mockReturnValueOnce({
        data: undefined,
        refetch: vi.fn(async () => undefined),
        startPolling: startExportPolling,
        stopPolling: stopExportPolling,
      });

    const { result } = renderHook(() => useRecovery("dpg-source"));

    expect(useQuery.mock.calls[1]?.[1]).toMatchObject({ skip: true });
    expect(startExportPolling).not.toHaveBeenCalled();
    expect(stopExportPolling).toHaveBeenCalled();
    expect(result.current.error).toBe(infoError);
  });

  it("returns the recovered database id from the mutation", async () => {
    recoverMutation.mockResolvedValue({
      data: { recoverDatabase: { id: "dpg-recovered-id" } },
    });
    const { result } = renderHook(() => useRecovery("dpg-source"));

    let recoveredId: string | null | undefined;
    await act(async () => {
      recoveredId = await result.current.recover({
        name: "recovered",
        targetTime: "2026-07-17T12:00:00Z",
      });
    });

    expect(recoveredId).toBe("dpg-recovered-id");
    expect(recoverMutation).toHaveBeenCalledWith({
      variables: {
        id: "dpg-source",
        name: "recovered",
        targetTime: "2026-07-17T12:00:00Z",
      },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Restoring into recovered…");
  });

  it("returns null instead of inventing an id when recovery omits it", async () => {
    recoverMutation.mockResolvedValue({
      data: { recoverDatabase: { id: null } },
    });
    const { result } = renderHook(() => useRecovery("dpg-source"));

    let recoveredId: string | null | undefined;
    await act(async () => {
      recoveredId = await result.current.recover({ name: "recovered" });
    });

    expect(recoveredId).toBeNull();
    expect(toastSuccess).not.toHaveBeenCalled();
    // The client-side guard's text is for developers, not the toast.
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't restore. Please try again.",
    );
  });
});

// w7/m453: the plan's recovery window rides on recovery info, and a target
// outside it is refused with a coded reason the restore toast shows as is.
describe("useRecovery plan window", () => {
  it("maps the plan's window, and none where no plan applies", () => {
    useQuery.mockReturnValueOnce({
      data: {
        databaseRecoveryInfo: {
          enabled: true,
          earliestRecoveryTime: "2026-11-13T12:00:00Z",
          latestRecoveryTime: "2026-11-16T03:00:00Z",
          backups: [],
          retention: {
            plan: "hobby",
            days: 3,
            upgradePlan: "pro",
            upgradeDays: 7,
          },
        },
      },
      loading: false,
    });
    const { result } = renderHook(() => useRecovery("dpg-source"));
    expect(result.current.info.retention).toEqual({
      plan: "hobby",
      days: 3,
      upgradePlan: "pro",
      upgradeDays: 7,
    });

    useQuery.mockReturnValue({
      data: {
        databaseRecoveryInfo: {
          enabled: true,
          earliestRecoveryTime: null,
          latestRecoveryTime: null,
          backups: [],
          retention: null,
        },
      },
      loading: false,
      refetch: vi.fn(async () => undefined),
      startPolling: vi.fn(),
      stopPolling,
    });
    useMutation
      .mockReturnValueOnce([createExportMutation, { loading: false }])
      .mockReturnValueOnce([recoverMutation, { loading: false }]);
    expect(
      renderHook(() => useRecovery("dpg-source")).result.current.info.retention,
    ).toBeNull();
  });

  it.each([
    [
      "RECOVERY_TARGET_BEFORE_WINDOW",
      "targetTime is before the earliest restore point (2026-11-13T12:00:00Z)",
      "TargetTime is before the earliest restore point (2026-11-13T12:00:00Z)",
    ],
    [
      "RECOVERY_TARGET_TOO_RECENT",
      "targetTime is within ten minutes of now; choose a time no later than 2026-11-16T11:50:00Z",
      "TargetTime is within ten minutes of now; choose a time no later than 2026-11-16T11:50:00Z",
    ],
  ])("shows a %s refusal's own message", async (code, message, shown) => {
    recoverMutation.mockRejectedValue(
      new CombinedGraphQLErrors({
        data: null,
        errors: [{ message, extensions: { code } }],
      }),
    );
    const { result } = renderHook(() => useRecovery("dpg-source"));
    let recoveredId: string | null | undefined;
    await act(async () => {
      recoveredId = await result.current.recover({
        name: "recovered",
        targetTime: "2026-11-12T12:00:00Z",
      });
    });
    expect(recoveredId).toBeNull();
    expect(toastError).toHaveBeenCalledWith(shown);
  });
});

/** The names of every field a document selects, at any depth. */
function selectedFields(doc: DocumentNode): string[] {
  const names: string[] = [];
  const walk = (set: SelectionSetNode | undefined) => {
    for (const sel of set?.selections ?? []) {
      if (sel.kind === Kind.FIELD) {
        names.push(sel.name.value);
        walk(sel.selectionSet);
      }
    }
  };
  for (const def of doc.definitions) {
    if (def.kind === Kind.OPERATION_DEFINITION) walk(def.selectionSet);
  }
  return names;
}

describe("useRecovery export downloads (w21/105)", () => {
  const item = {
    id: "exp-1",
    status: "available",
    createdAt: "2026-07-14T11:00:00Z",
    expiresAt: "2026-07-21T11:00:00Z",
    filename: "dump.dir.tar.gz",
    failureReason: null,
  };

  // Pressing Download changes state, so the hook renders again: both mutations
  // must answer on every render, not once each as the tests above arrange.
  beforeEach(() => {
    useMutation.mockReset();
    useMutation.mockReturnValue([vi.fn(), { loading: false }]);
  });

  it("keeps the url out of the status poll and puts it in the download query", () => {
    // bex-api mints a URL, and records one download, for every query that
    // selects `url`: the 25-second poll must not.
    expect(selectedFields(DatabaseExportsDocument)).not.toContain("url");
    expect(selectedFields(DatabaseExportsDocument)).not.toContain(
      "urlExpiresAt",
    );
    expect(selectedFields(DatabaseExportLinksDocument)).toContain("url");
  });

  it("asks for a fresh link when pressed and hands it to the browser", async () => {
    clientQuery.mockResolvedValue({
      data: {
        databaseExports: [
          { id: "exp-0", url: "https://objects.example.com/other" },
          { id: "exp-1", url: "https://objects.example.com/signed" },
        ],
      },
    });
    const { result } = renderHook(() => useRecovery("dpg-source"));

    await act(async () => {
      await result.current.downloadExport(item);
    });

    expect(clientQuery).toHaveBeenCalledTimes(1);
    expect(clientQuery.mock.calls[0]?.[0]).toMatchObject({
      query: DatabaseExportLinksDocument,
      variables: { id: "dpg-source" },
      fetchPolicy: "network-only",
    });
    expect(openDownloadURL).toHaveBeenCalledWith(
      "https://objects.example.com/signed",
      "dump.dir.tar.gz",
    );
    expect(result.current.downloadingExportId).toBeNull();
  });

  it("says so and refreshes the list when the export has no link any more", async () => {
    clientQuery.mockResolvedValue({
      data: { databaseExports: [{ id: "exp-1", url: null }] },
    });
    const { result } = renderHook(() => useRecovery("dpg-source"));

    await act(async () => {
      await result.current.downloadExport(item);
    });

    expect(openDownloadURL).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith(
      "This export is no longer available for download.",
    );
  });

  it("toasts a refusal and downloads nothing", async () => {
    clientQuery.mockRejectedValue(new Error("Failed to fetch"));
    const { result } = renderHook(() => useRecovery("dpg-source"));

    await act(async () => {
      await result.current.downloadExport(item);
    });

    expect(openDownloadURL).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't get the download link. Please try again.",
    );
    expect(result.current.downloadingExportId).toBeNull();
  });
});
