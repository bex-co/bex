import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RuntimeSection } from "@/features/services/components/runtime-section";

// Stub the mutation hook so tests don't need an Apollo context.
const mockSetRuntime = vi.fn().mockResolvedValue(true);
vi.mock("@/features/services/hooks/use-runtime", () => ({
  useRuntime: () => ({ setRuntime: mockSetRuntime, saving: false }),
}));

function picker() {
  return screen.getByRole("combobox", { name: "Runtime" });
}

describe("RuntimeSection", () => {
  beforeEach(() => {
    mockSetRuntime.mockClear();
  });

  it("shows the current runtime and the rebuild note", () => {
    render(<RuntimeSection serviceId="web" runtime="node" builder="native" />);
    expect(picker()).toHaveTextContent("Node");
    expect(
      screen.getByText(
        "Changing the runtime rebuilds and redeploys the service. Native runtimes use the Build Command and Start Command below.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("offers Render's runtimes in Render's order", async () => {
    const user = userEvent.setup();
    render(<RuntimeSection serviceId="web" runtime="node" builder="native" />);
    await user.click(picker());
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Docker",
      "Elixir",
      "Go",
      "Node",
      "Python 3",
      "Ruby",
      "Rust",
    ]);
  });

  it("reads a legacy Dockerfile build with no runtime as Docker", () => {
    render(<RuntimeSection serviceId="web" runtime="" builder="dockerfile" />);
    expect(picker()).toHaveTextContent("Docker");
  });

  it("calls setRuntime with the selected runtime only after confirming", async () => {
    const user = userEvent.setup();
    render(
      <RuntimeSection serviceId="web" runtime="docker" builder="docker" />,
    );

    await user.click(picker());
    await user.click(screen.getByRole("option", { name: "Python 3" }));
    const save = screen.getByRole("button", { name: "Save" });
    expect(save).toBeEnabled();

    await user.click(save);
    // The confirmation appears first; nothing is saved yet.
    expect(mockSetRuntime).not.toHaveBeenCalled();
    expect(
      screen.getByRole("heading", {
        name: "Change the runtime to Python 3?",
      }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Change runtime" }));
    expect(mockSetRuntime).toHaveBeenCalledWith("web", "python");
  });

  it("saves nothing when the confirmation is cancelled", async () => {
    const user = userEvent.setup();
    render(
      <RuntimeSection serviceId="web" runtime="docker" builder="docker" />,
    );

    await user.click(picker());
    await user.click(screen.getByRole("option", { name: "Go" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(mockSetRuntime).not.toHaveBeenCalled();
  });
});
