import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BuildDeploySection } from "@/features/services/components/build-deploy-section";

const setRootDir = vi.fn(async () => true);
const setAutoDeploy = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-root-dir", () => ({
  useRootDir: () => ({ setRootDir, busy: false }),
}));

vi.mock("@/features/services/hooks/use-auto-deploy", () => ({
  useAutoDeploy: () => ({ setAutoDeploy, busy: false }),
}));

const connectionState: {
  connection:
    | { connected: boolean; accountLogin: string; installUrl: string }
    | undefined;
} = { connection: undefined };

vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnection: () => connectionState,
}));

beforeEach(() => {
  setRootDir.mockClear();
  setRootDir.mockResolvedValue(true);
  setAutoDeploy.mockClear();
  setAutoDeploy.mockResolvedValue(true);
  connectionState.connection = undefined;
});

describe("BuildDeploySection", () => {
  it("shows the repo and branch read-only", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        autoDeploy={false}
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
        autoDeploy={false}
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
        autoDeploy={false}
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
        autoDeploy={false}
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
        autoDeploy={false}
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
        autoDeploy={false}
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
        autoDeploy={false}
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

  it("flipping Auto-Deploy on calls setAutoDeploy(id, true)", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        autoDeploy={false}
      />,
    );

    await user.click(screen.getByRole("switch", { name: "Auto-Deploy" }));
    expect(setAutoDeploy).toHaveBeenCalledWith("app", true);
  });

  it("reverts the optimistic switch when setAutoDeploy fails", async () => {
    setAutoDeploy.mockResolvedValue(false);
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        autoDeploy={true}
      />,
    );

    const toggle = screen.getByRole("switch", { name: "Auto-Deploy" });
    expect(toggle).toBeChecked();
    await user.click(toggle);
    expect(setAutoDeploy).toHaveBeenCalledWith("app", false);
    expect(toggle).toBeChecked();
  });

  it("names the GitHub app as the push source when the repo is on the connected account", () => {
    connectionState.connection = {
      connected: true,
      accountLogin: "acme",
      installUrl: "",
    };
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/acme/mono"
        branch="main"
        rootDir={null}
        autoDeploy={true}
      />,
    );
    expect(
      screen.getByText(/redeploys automatically via the GitHub app/),
    ).toBeInTheDocument();
  });

  it("names the manual webhook as the push source when GitHub is not connected", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        autoDeploy={true}
      />,
    );
    expect(
      screen.getByText(/manual git webhook/),
    ).toBeInTheDocument();
  });
});
