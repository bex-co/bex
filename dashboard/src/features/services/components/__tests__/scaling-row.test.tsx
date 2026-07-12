import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ScalingRow } from "@/features/services/components/scaling-row";

const scaleService = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-scale-service", () => ({
  useScaleService: () => ({ scaleService, busy: false }),
}));

beforeEach(() => {
  scaleService.mockClear();
});

describe("ScalingRow", () => {
  it("renders the current replica count in the input", () => {
    render(<ScalingRow serviceId="app" replicas={3} />);
    const input = screen.getByRole("spinbutton");
    expect(input).toHaveValue(3);
  });

  it("falls back to 1 when replicas is null", () => {
    render(<ScalingRow serviceId="app" replicas={null} />);
    expect(screen.getByRole("spinbutton")).toHaveValue(1);
  });

  it("Save button is disabled when draft equals current count", () => {
    render(<ScalingRow serviceId="app" replicas={2} />);
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("Save button enables after incrementing", async () => {
    const user = userEvent.setup();
    render(<ScalingRow serviceId="app" replicas={2} />);
    await user.click(screen.getByRole("button", { name: "Increase instance count" }));
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("increment button is disabled at MAX_INSTANCES (100)", () => {
    render(<ScalingRow serviceId="app" replicas={100} />);
    expect(
      screen.getByRole("button", { name: "Increase instance count" }),
    ).toBeDisabled();
  });

  it("decrement button is disabled at MIN_INSTANCES (1)", () => {
    render(<ScalingRow serviceId="app" replicas={1} />);
    expect(
      screen.getByRole("button", { name: "Decrease instance count" }),
    ).toBeDisabled();
  });

  it("clicking Save calls scaleService with the new count", async () => {
    const user = userEvent.setup();
    render(<ScalingRow serviceId="svc-1" replicas={1} />);
    await user.click(screen.getByRole("button", { name: "Increase instance count" }));
    await user.click(screen.getByRole("button", { name: "Increase instance count" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(scaleService).toHaveBeenCalledWith("svc-1", 3);
  });

  it("resets draft to current count when scaleService rejects", async () => {
    scaleService.mockResolvedValueOnce(false);
    const user = userEvent.setup();
    render(<ScalingRow serviceId="app" replicas={2} />);
    await user.click(screen.getByRole("button", { name: "Increase instance count" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    // After rejection the draft should revert to the original value (2)
    expect(screen.getByRole("spinbutton")).toHaveValue(2);
  });
});
