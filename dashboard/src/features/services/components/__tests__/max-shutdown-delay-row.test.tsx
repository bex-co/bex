import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MaxShutdownDelayRow } from "@/features/services/components/max-shutdown-delay-row";

const setMaxShutdownDelay = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-max-shutdown-delay", () => ({
  useMaxShutdownDelay: () => ({ setMaxShutdownDelay, busy: false }),
}));

beforeEach(() => {
  setMaxShutdownDelay.mockClear();
  setMaxShutdownDelay.mockResolvedValue(true);
});

describe("MaxShutdownDelayRow", () => {
  it("shows the 30-second platform default when the field is absent", () => {
    render(
      <MaxShutdownDelayRow serviceId="web" maxShutdownDelaySeconds={null} />,
    );

    expect(screen.getByText("30 sec")).toBeInTheDocument();
    expect(screen.getByText(/1–300 seconds/)).toBeInTheDocument();
  });

  it("persists an in-range integer and refetches the service", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(
      <MaxShutdownDelayRow
        serviceId="worker"
        maxShutdownDelaySeconds={45}
        onChanged={onChanged}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit max shutdown delay" }),
    );
    const input = screen.getByRole("spinbutton", {
      name: "Max shutdown delay",
    });
    await user.clear(input);
    await user.type(input, "120");
    await user.click(
      screen.getByRole("button", { name: "Save max shutdown delay" }),
    );

    expect(setMaxShutdownDelay).toHaveBeenCalledWith("worker", 120);
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("does not allow fractions or values outside 1-300", async () => {
    const user = userEvent.setup();
    render(
      <MaxShutdownDelayRow serviceId="web" maxShutdownDelaySeconds={30} />,
    );
    await user.click(
      screen.getByRole("button", { name: "Edit max shutdown delay" }),
    );
    const input = screen.getByRole("spinbutton", {
      name: "Max shutdown delay",
    });
    const save = screen.getByRole("button", {
      name: "Save max shutdown delay",
    });

    for (const value of ["0", "301", "1.5"]) {
      await user.clear(input);
      await user.type(input, value);
      expect(save).toBeDisabled();
    }
    expect(setMaxShutdownDelay).not.toHaveBeenCalled();
  });
});
