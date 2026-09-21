import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServicePortRow } from "@/features/services/components/service-port-row";

const setPort = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-port", () => ({
  usePort: () => ({ setPort, busy: false }),
}));

beforeEach(() => {
  setPort.mockClear();
  setPort.mockResolvedValue(true);
});

describe("ServicePortRow", () => {
  it("shows the service's current port", () => {
    render(<ServicePortRow serviceId="web" port={8080} />);

    expect(screen.getByRole("spinbutton", { name: "Port" })).toHaveValue(8080);
  });

  // The write verb this milestone shipped (w4/m121/t001) reaching the UI: the
  // wizard could only ever pick a port at create, so an image binding anything
  // but 3000 was a permanent 503 with no dashboard remedy.
  it("saves the edited port as a number", async () => {
    const user = userEvent.setup();
    render(<ServicePortRow serviceId="web" port={3000} />);

    await user.click(screen.getByRole("button", { name: "Edit Port" }));
    const input = screen.getByRole("spinbutton", { name: "Port" });
    await user.clear(input);
    await user.type(input, "8080");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    expect(setPort).toHaveBeenCalledWith("web", 8080);
  });

  it("blocks a port the container could never bind instead of calling the API", async () => {
    const user = userEvent.setup();
    render(<ServicePortRow serviceId="web" port={3000} />);

    await user.click(screen.getByRole("button", { name: "Edit Port" }));
    const input = screen.getByRole("spinbutton", { name: "Port" });
    await user.clear(input);
    await user.type(input, "80");

    expect(
      screen.getByText("Enter a port between 1024 and 65535."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /save changes/i }),
    ).toBeDisabled();
    expect(setPort).not.toHaveBeenCalled();
  });

  it("renders an absent port as empty rather than as 0", () => {
    render(<ServicePortRow serviceId="web" port={null} />);

    expect(screen.getByRole("spinbutton", { name: "Port" })).toHaveValue(null);
  });
});
