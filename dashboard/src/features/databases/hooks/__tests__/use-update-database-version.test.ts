import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastWarning = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    warning: (...args: unknown[]) => toastWarning(...args),
  },
}));

import { useUpdateDatabaseVersion } from "@/features/databases/hooks/use-update-database-version";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastWarning.mockReset();
});

function accepted(upgradeNotice: string | null) {
  return vi.fn().mockResolvedValue({
    data: {
      updateDatabaseVersion: {
        id: "dpg-orders",
        version: "16",
        status: "upgrading",
        upgradeNotice,
      },
    },
  });
}

describe("useUpdateDatabaseVersion", () => {
  it("confirms an accepted upgrade that drops nothing", async () => {
    mockUseMutation.mockReturnValue([accepted(null)]);
    const { result } = renderHook(() => useUpdateDatabaseVersion());

    let ok = false;
    await act(async () => {
      ok = await result.current.updateVersion("dpg-orders", "17");
    });

    expect(ok).toBe(true);
    expect(toastSuccess).toHaveBeenCalledWith(
      "Upgrade to PostgreSQL 17 started.",
    );
    expect(toastWarning).not.toHaveBeenCalled();
  });

  // w34/m543: an upgrade from before PostgreSQL 17 drops a publisher's
  // logical replication slots, and the server says so; the toast warns with
  // its words and stays until dismissed.
  it("warns with the server's notice when the upgrade drops slots", async () => {
    const notice =
      "PostgreSQL 16's pg_upgrade does not carry logical replication slots, so this upgrade drops cdc";
    mockUseMutation.mockReturnValue([accepted(notice)]);
    const { result } = renderHook(() => useUpdateDatabaseVersion());

    await act(async () => {
      await result.current.updateVersion("dpg-orders", "17");
    });

    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastWarning).toHaveBeenCalledWith(
      "Upgrade to PostgreSQL 17 started.",
      { description: notice, duration: Infinity },
    );
  });

  // A lagging slot's refusal reaches the dialog's inline error as the
  // server words it, naming the slot.
  it("surfaces a lagging-slot refusal inline", async () => {
    const message =
      "PostgreSQL's pg_upgrade refuses to upgrade PostgreSQL 17 while a logical replication slot has not consumed all WAL: cdc (no subscriber is connected to it)";
    mockUseMutation.mockReturnValue([
      vi
        .fn()
        .mockRejectedValue(
          codedGraphQLError(
            "POSTGRES_UPGRADE_LOGICAL_SLOTS_BEHIND",
            {},
            message,
          ),
        ),
    ]);
    const { result } = renderHook(() => useUpdateDatabaseVersion());

    let ok = true;
    await act(async () => {
      ok = await result.current.updateVersion("dpg-orders", "18");
    });

    expect(ok).toBe(false);
    expect(result.current.error).toContain("cdc (no subscriber is connected");
    expect(toastSuccess).not.toHaveBeenCalled();
  });
});
