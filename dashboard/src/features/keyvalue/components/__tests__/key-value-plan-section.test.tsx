import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KeyValuePlanSection } from "@/features/keyvalue/components/key-value-plan-section";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { KeyValueView } from "@/features/keyvalue/types";

const updatePlan = vi.fn();

vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () => ({
    instanceTypes: [
      { id: "free", name: "Free", cpu: "0.1", memory: "256Mi", storageGB: 1 },
      {
        id: "starter",
        name: "Starter",
        cpu: "0.5",
        memory: "1Gi",
        storageGB: 10,
      },
    ],
    loading: false,
    error: undefined,
  }),
}));

vi.mock("@/features/keyvalue/hooks/use-update-key-value-plan", () => ({
  useUpdateKeyValuePlan: () => ({ updatePlan, busy: false }),
}));

const KEY_VALUE: KeyValueView = {
  id: "red-sessions",
  name: "sessions",
  status: "available",
  plan: "free",
  version: "8",
  createdAt: null,
  externalHost: null,
  public: false,
  suspended: false,
};

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  updatePlan.mockReset().mockResolvedValue(true);
});

describe("KeyValuePlanSection", () => {
  it("lets a contributor change plan through can_operate", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={onChanged} />);

    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(updatePlan).toHaveBeenCalledWith(
      "red-sessions",
      "starter",
      "Starter",
    );
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("disables plan controls for a viewer with the operate reason", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canOperate: false }),
    );
    const user = userEvent.setup();
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />);

    const starter = screen.getByRole("radio", { name: /Starter/ });
    expect(starter).toBeDisabled();
    expect(
      screen.getByText(/Your role can only view this service/),
    ).toBeInTheDocument();
    await user.click(starter);
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("does not disable before capabilities are definitive", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canOperate: false, loading: true, loaded: false }),
    );
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />);
    expect(screen.getByRole("radio", { name: /Starter/ })).toBeEnabled();
  });
});
