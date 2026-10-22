import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
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

  it("blocks a port outside 1-65535 instead of calling the API", async () => {
    const user = userEvent.setup();
    render(<ServicePortRow serviceId="web" port={3000} />);

    await user.click(screen.getByRole("button", { name: "Edit Port" }));
    const input = screen.getByRole("spinbutton", { name: "Port" });
    await user.clear(input);
    await user.type(input, "70000");

    expect(
      screen.getByText("Enter a port between 1 and 65535."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /save changes/i }),
    ).toBeDisabled();
    expect(setPort).not.toHaveBeenCalled();
  });

  // w6/m149: every pod lets an unprivileged process bind ports below 1024, as
  // on Render, so :80 is a valid port to save.
  it("saves a port below 1024", async () => {
    const user = userEvent.setup();
    render(<ServicePortRow serviceId="web" port={3000} />);

    await user.click(screen.getByRole("button", { name: "Edit Port" }));
    const input = screen.getByRole("spinbutton", { name: "Port" });
    await user.clear(input);
    await user.type(input, "80");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    expect(setPort).toHaveBeenCalledWith("web", 80);
  });

  // w4/278: "03000" parses to the current port, yet the text comparison enabled
  // Save and each save rolled an identical deploy.
  it("keeps Save disabled for a zero-padded spelling of the current port", async () => {
    const user = userEvent.setup();
    render(<ServicePortRow serviceId="web" port={3000} />);

    await user.click(screen.getByRole("button", { name: "Edit Port" }));
    const input = screen.getByRole("spinbutton", { name: "Port" });
    const save = screen.getByRole("button", { name: /save changes/i });
    // A browser keeps "03000" in a number input; user-event drops the leading
    // zero, so set the draft the way the browser reports it.
    for (const padded of ["03000", "003000"]) {
      fireEvent.change(input, { target: { value: padded } });
      expect(input).toHaveDisplayValue(padded);
      expect(save, padded).toBeDisabled();
    }

    fireEvent.change(input, { target: { value: "03001" } });
    await user.click(save);
    expect(setPort).toHaveBeenCalledExactlyOnceWith("web", 3001);
  });

  it("saves a valid port when the current one is unknown", async () => {
    const user = userEvent.setup();
    render(<ServicePortRow serviceId="web" port={null} />);

    await user.click(screen.getByRole("button", { name: "Edit Port" }));
    await user.type(screen.getByRole("spinbutton", { name: "Port" }), "3000");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    expect(setPort).toHaveBeenCalledWith("web", 3000);
  });

  it("renders an absent port as empty rather than as 0", () => {
    render(<ServicePortRow serviceId="web" port={null} />);

    expect(screen.getByRole("spinbutton", { name: "Port" })).toHaveValue(null);
  });

  // w6/m148: an image-port private service publishes every TCP port its image
  // declares, so the port set here picks the primary, not the only listener.
  it("explains that an image-port service's other declared ports stay published", () => {
    render(<ServicePortRow serviceId="ch" port={8123} imagePorts />);

    expect(
      screen.getByText(
        /Setting a port here selects the primary listener .* while the image's other declared TCP ports stay published\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/The port your container listens on/),
    ).not.toBeInTheDocument();
  });

  it("does not suggest the default port while an image's ports are unknown", () => {
    const { unmount } = render(<ServicePortRow serviceId="web" port={null} />);
    expect(screen.getByRole("spinbutton", { name: "Port" })).toHaveAttribute(
      "placeholder",
      "10000",
    );
    unmount();

    render(<ServicePortRow serviceId="ch" port={null} imagePorts />);
    expect(
      screen.getByRole("spinbutton", { name: "Port" }),
    ).not.toHaveAttribute("placeholder");
  });

  it("keeps the configured-port hint for every other service", () => {
    render(<ServicePortRow serviceId="web" port={8080} />);

    expect(
      screen.getByText(/The port your container listens on \(default 10000\)/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/other declared TCP ports/),
    ).not.toBeInTheDocument();
  });
});
