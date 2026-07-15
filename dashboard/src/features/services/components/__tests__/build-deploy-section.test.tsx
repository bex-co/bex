import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BuildDeploySection } from "@/features/services/components/build-deploy-section";

const setRootDir = vi.fn(async () => true);
const setAutoDeploy = vi.fn(async () => true);
const setPreDeployCommand = vi.fn(async () => true);
const setBuildFilter = vi.fn(async () => true);
const setStartCommand = vi.fn(async () => true);
const setDockerfilePath = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-root-dir", () => ({
  useRootDir: () => ({ setRootDir, busy: false }),
}));

vi.mock("@/features/services/hooks/use-build-filter", () => ({
  useBuildFilter: () => ({ setBuildFilter, busy: false }),
}));

vi.mock("@/features/services/hooks/use-start-command", () => ({
  useStartCommand: () => ({ setStartCommand, busy: false }),
}));

vi.mock("@/features/services/hooks/use-dockerfile-path", () => ({
  useDockerfilePath: () => ({ setDockerfilePath, busy: false }),
}));

vi.mock("@/features/services/hooks/use-pre-deploy-command", () => ({
  usePreDeployCommand: () => ({ setPreDeployCommand, busy: false }),
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
  setPreDeployCommand.mockClear();
  setPreDeployCommand.mockResolvedValue(true);
  setBuildFilter.mockClear();
  setBuildFilter.mockResolvedValue(true);
  setStartCommand.mockClear();
  setStartCommand.mockResolvedValue(true);
  setDockerfilePath.mockClear();
  setDockerfilePath.mockResolvedValue(true);
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
        preDeployCommand={null}
        showPreDeployCommand={false}
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
        preDeployCommand={null}
        showPreDeployCommand={false}
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
        preDeployCommand={null}
        showPreDeployCommand={false}
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
        preDeployCommand={null}
        showPreDeployCommand={false}
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
        preDeployCommand={null}
        showPreDeployCommand={false}
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
      within(dialog).getByText("Change Root Directory to the repository root?"),
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
        preDeployCommand={null}
        showPreDeployCommand={false}
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
        preDeployCommand={null}
        showPreDeployCommand={false}
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

  it("edits and confirms the native Start Command", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        runtime="node"
        startCommand="npm start"
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
        showStartCommand
      />,
    );

    expect(screen.getByText("Start Command")).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Edit Start Command" }),
    );
    const input = screen.getByDisplayValue("npm start");
    await user.clear(input);
    await user.type(input, "node server.js");
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(setStartCommand).toHaveBeenCalledWith("app", "node server.js");
  });

  it("clears the persisted Start Command", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        runtime="node"
        startCommand="npm start"
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
        showStartCommand
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit Start Command" }),
    );
    await user.clear(screen.getByDisplayValue("npm start"));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(setStartCommand).toHaveBeenCalledWith("app", "");
  });

  it("shows Dockerfile Path only for Dockerfile builds and persists an edit", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        runtime="node"
        dockerfilePath="stale/Dockerfile"
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(screen.queryByText("Dockerfile Path")).not.toBeInTheDocument();

    rerender(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        runtime={null}
        builder="dockerfile"
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(screen.getByText("Dockerfile Path")).toBeInTheDocument();

    rerender(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        runtime="docker"
        startCommand={null}
        dockerfilePath="Dockerfile"
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
        showStartCommand
      />,
    );
    expect(screen.getByText("Docker Command")).toBeInTheDocument();
    expect(screen.getByText("Dockerfile Path")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Edit Dockerfile Path" }),
    );
    const input = screen.getByDisplayValue("Dockerfile");
    await user.clear(input);
    await user.type(input, "docker/Dockerfile.prod");
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(setDockerfilePath).toHaveBeenCalledWith(
      "app",
      "docker/Dockerfile.prod",
    );
  });

  it("keeps the Dockerfile Path draft after a failed mutation", async () => {
    setDockerfilePath.mockResolvedValue(false);
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        runtime="docker"
        dockerfilePath="Dockerfile"
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit Dockerfile Path" }),
    );
    const input = screen.getByDisplayValue("Dockerfile");
    await user.clear(input);
    await user.type(input, "docker/Dockerfile.prod");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await user.click(
      within(await screen.findByRole("alertdialog")).getByRole("button", {
        name: "Save",
      }),
    );

    expect(screen.getByDisplayValue("docker/Dockerfile.prod")).toBeVisible();
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
        preDeployCommand={null}
        showPreDeployCommand={false}
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
        preDeployCommand={null}
        showPreDeployCommand={false}
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
        preDeployCommand={null}
        showPreDeployCommand={false}
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
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(screen.getByText(/manual git webhook/)).toBeInTheDocument();
  });

  // Pre-Deploy Command field (w1/m33).
  it("hides the Pre-Deploy Command field when showPreDeployCommand is false", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        autoDeploy={false}
        preDeployCommand="npm run migrate"
        showPreDeployCommand={false}
      />,
    );
    expect(screen.queryByText("Pre-Deploy Command")).not.toBeInTheDocument();
    expect(screen.queryByText("npm run migrate")).not.toBeInTheDocument();
  });

  it("shows the current pre-deploy command, or an empty state when unset", () => {
    const { unmount } = render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        autoDeploy={false}
        preDeployCommand="npm run migrate"
        showPreDeployCommand={true}
      />,
    );
    expect(screen.getByText("Pre-Deploy Command")).toBeInTheDocument();
    expect(screen.getByText("npm run migrate")).toBeInTheDocument();
    unmount();

    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={true}
      />,
    );
    expect(screen.getByText("No pre-deploy command")).toBeInTheDocument();
  });

  it("edits and saves the pre-deploy command via setPreDeployCommand (no confirm)", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={true}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit Pre-Deploy Command" }),
    );
    const input = screen.getByPlaceholderText("e.g. npm run migrate");
    await user.type(input, "  npm run migrate  ");
    await user.click(screen.getByRole("button", { name: "Save" }));

    // Trimmed before the mutation; no confirm dialog for this field.
    expect(setPreDeployCommand).toHaveBeenCalledWith("app", "npm run migrate");
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  // Build Filters editor (w1/m34).
  it("renders the Build Filters editor with both path lists", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(screen.getByText("Included Paths")).toBeInTheDocument();
    expect(screen.getByText("Ignored Paths")).toBeInTheDocument();
    // Save is disabled until the draft differs from the (empty) current filter.
    expect(
      screen.getByRole("button", { name: "Save Build Filters" }),
    ).toBeDisabled();
  });

  it("shows the existing build-filter globs", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        buildFilter={{ paths: ["src/**"], ignoredPaths: ["docs/**"] }}
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(screen.getByDisplayValue("src/**")).toBeInTheDocument();
    expect(screen.getByDisplayValue("docs/**")).toBeInTheDocument();
  });

  it("adds an included path and saves via setBuildFilter", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Add included path" }));
    await user.type(screen.getByPlaceholderText("e.g. src/**"), "src/**");
    await user.click(
      screen.getByRole("button", { name: "Save Build Filters" }),
    );

    expect(setBuildFilter).toHaveBeenCalledWith("app", ["src/**"], []);
  });

  it("removes an existing ignored path and saves the shortened list", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        buildFilter={{ paths: ["src/**"], ignoredPaths: ["docs/**"] }}
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Remove ignored path" }),
    );
    await user.click(
      screen.getByRole("button", { name: "Save Build Filters" }),
    );

    // Ignored list cleared, included list preserved.
    expect(setBuildFilter).toHaveBeenCalledWith("app", ["src/**"], []);
  });
});
