import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ManualScalingSection } from "@/features/services/components/manual-scaling-section";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { CREATE_REASON, mockCapabilities } from "@/test/mocks/capabilities";

const scaleService = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-scale-service", () => ({
  useScaleService: () => ({ scaleService, busy: false }),
}));

beforeEach(() => {
  scaleService.mockClear();
});

afterEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

// The instance count is a service setting: can_create, as Render's role table
// gives a Contributor no "Modify service configuration" (w9/m619).
describe("ManualScalingSection role gating (w9/m619)", () => {
  it("holds the instance count for a contributor with the role's reason", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const user = userEvent.setup();
    render(<ManualScalingSection serviceId="web" replicas={2} plan="pro" />);

    const count = screen.getByRole("spinbutton", { name: "Instances" });
    expect(count).toBeDisabled();
    const save = screen.getByRole("button", { name: "Save Changes" });
    expect(save).toBeDisabled();
    expect(save.closest('[data-slot="tooltip-trigger"]')).not.toBeNull();
    expect(screen.getByText(CREATE_REASON)).toBeInTheDocument();
    await user.click(save);
    expect(scaleService).not.toHaveBeenCalled();
  });

  it("lets a developer change the instance count", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    const user = userEvent.setup();
    render(<ManualScalingSection serviceId="web" replicas={2} plan="pro" />);

    const count = screen.getByRole("spinbutton", { name: "Instances" });
    expect(count).toBeEnabled();
    fireEvent.change(count, { target: { value: "4" } });
    await user.click(screen.getByRole("button", { name: "Save Changes" }));
    expect(scaleService).toHaveBeenCalledWith("web", 4);
    expect(screen.queryByText(CREATE_REASON)).toBeNull();
  });
});
