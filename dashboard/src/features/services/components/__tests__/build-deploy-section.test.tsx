import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BuildDeploySection } from "@/features/services/components/build-deploy-section";

const setRootDir = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-root-dir", () => ({
  useRootDir: () => ({ setRootDir, busy: false }),
}));

beforeEach(() => {
  setRootDir.mockClear();
  setRootDir.mockResolvedValue(true);
});

describe("BuildDeploySection", () => {
  it("shows the repo and branch read-only", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
      />,
    );
    expect(screen.getByText("https://github.com/x/mono")).toBeInTheDocument();
    expect(screen.getByText("main")).toBeInTheDocument();
  });

  it("shows an honest 'repository root' state when rootDir is unset", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
      />,
    );
    expect(screen.getByText("Repository root")).toBeInTheDocument();
  });

  it("shows the current rootDir when set", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir="backend"
      />,
    );
    expect(screen.getByText("backend")).toBeInTheDocument();
  });

  it("edit -> confirm -> setRootDir with the new value", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit Root Directory" }),
    );
    const input = screen.getByPlaceholderText("e.g. backend");
    await user.type(input, "backend");
    await user.click(screen.getByRole("button", { name: "Save" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText("Change Root Directory to backend?"),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(setRootDir).toHaveBeenCalledWith("app", "backend");
  });

  it("confirm dialog names 'repository root' when clearing to empty", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir="backend"
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit Root Directory" }),
    );
    const input = screen.getByDisplayValue("backend");
    await user.clear(input);
    await user.click(screen.getByRole("button", { name: "Save" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText(
        "Change Root Directory to the repository root?",
      ),
    ).toBeInTheDocument();
  });

  it("Save is disabled until the draft differs from the current value", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir="backend"
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit Root Directory" }),
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("cancel discards the draft without calling setRootDir", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit Root Directory" }),
    );
    await user.type(screen.getByPlaceholderText("e.g. backend"), "backend");
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(setRootDir).not.toHaveBeenCalled();
    expect(screen.getByText("Repository root")).toBeInTheDocument();
  });
});
