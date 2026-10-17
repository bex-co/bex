import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KeyValuePlanSection } from "@/features/keyvalue/components/key-value-plan-section";
import { DatabasePlanSection } from "@/features/databases/components/database-plan-section";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { KeyValueView } from "@/features/keyvalue/types";
import type { DatabaseDetailView } from "@/features/databases/types";

// w4/m218 (a796): both datastore detail plan editors dropped the catalog
// hook's error and drew an empty radio group. A settled failure now explains
// itself and re-reads in place; nothing is saved by Retry.
const retry = vi.fn(async () => undefined);
const catalog = vi.hoisted(() => ({
  instanceTypes: [] as Array<Record<string, unknown>>,
  loading: false,
  error: undefined as Error | undefined,
  retrying: false,
}));
const updatePlan = vi.fn();

vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: vi.fn(),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () => ({ ...catalog, retry }),
}));
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({ ...catalog, retry }),
}));
vi.mock("@/features/keyvalue/hooks/use-update-key-value-plan", () => ({
  useUpdateKeyValuePlan: () => ({ updatePlan, busy: false }),
}));
vi.mock("@/features/databases/hooks/use-update-database-plan", () => ({
  useUpdateDatabasePlan: () => ({ updatePlan, busy: false }),
}));

const KEY_VALUE = {
  id: "red-sessions",
  name: "sessions",
  status: "available",
  plan: "free",
  version: "8",
  createdAt: null,
  externalHost: null,
  public: false,
  suspended: false,
} as KeyValueView;
const DATABASE = {
  id: "dpg-shop",
  name: "shop",
  status: "available",
  plan: "free",
  version: "18",
  diskSizeGB: 1,
  createdAt: null,
  public: false,
  suspended: "not_suspended",
  databaseName: "shop",
  databaseUser: "shop_user",
  highAvailabilityEnabled: false,
  diskAutoscalingEnabled: false,
  poolerEnabled: false,
  readReplicas: [],
  externalHost: null,
  backupsEnabled: false,
} as DatabaseDetailView;

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  catalog.instanceTypes = [];
  catalog.loading = false;
  catalog.error = new Error("catalog unavailable");
  catalog.retrying = false;
  retry.mockClear();
  updatePlan.mockReset();
});

describe.each([
  [
    "Key Value",
    () =>
      render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />),
  ],
  [
    "Postgres",
    () =>
      render(<DatabasePlanSection database={DATABASE} onChanged={vi.fn()} />),
  ],
])("%s detail plan editor with a failed catalog (w4/m218)", (_, mount) => {
  it("explains the failure and retries the read in place", async () => {
    const user = userEvent.setup();
    mount();
    expect(screen.getByRole("alert")).toHaveTextContent("Plans couldn't load");
    expect(screen.queryByRole("radiogroup")).toBeNull();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(retry).toHaveBeenCalledOnce();
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("shows a busy, unclickable Retry while the read is in flight", () => {
    catalog.retrying = true;
    mount();
    expect(screen.getByRole("button", { name: "Try again" })).toBeDisabled();
  });
});
