import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
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

    const input = screen.getByRole("spinbutton", {
      name: "Max shutdown delay",
    });
    expect(input).toHaveValue(30);
    expect(input).toBeDisabled();
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
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    // The save starts a deploy, so it confirms first (w4/616).
    const dialog = screen.getByRole("alertdialog");
    expect(
      within(dialog).getByText("Change the Max shutdown delay to 120 seconds?"),
    ).toBeInTheDocument();
    expect(setMaxShutdownDelay).not.toHaveBeenCalled();
    await user.click(
      within(dialog).getByRole("button", { name: "Save changes" }),
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
    const save = screen.getByRole("button", { name: "Save changes" });

    for (const value of ["0", "301", "1.5"]) {
      await user.clear(input);
      await user.type(input, value);
      expect(save).toBeDisabled();
      // w4/494: Save is not simply greyed out; the rule is stated, announced
      // and linked to the input.
      const message = screen.getByRole("alert");
      expect(message).toHaveTextContent(
        "Enter a whole number of seconds from 1 to 300.",
      );
      expect(input).toHaveAccessibleDescription(
        expect.stringContaining("Enter a whole number of seconds"),
      );
    }
    expect(setMaxShutdownDelay).not.toHaveBeenCalled();

    await user.clear(input);
    await user.type(input, "60");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(save).toBeEnabled();
  });
});
