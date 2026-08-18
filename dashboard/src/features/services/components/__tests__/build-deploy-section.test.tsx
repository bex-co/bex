import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BuildDeploySection } from "@/features/services/components/build-deploy-section";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

const setRootDir = vi.fn(async () => true);
const setBranch = vi.fn(async () => true);
const setAutoDeploy = vi.fn(async () => true);
const setPreDeployCommand = vi.fn(async () => true);
const setBuildFilter = vi.fn(async () => true);
const setBuildCommand = vi.fn(async () => true);
const setStartCommand = vi.fn(async () => true);
const setDockerfilePath = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-root-dir", () => ({
  useRootDir: () => ({ setRootDir, busy: false }),
}));

vi.mock("@/features/services/hooks/use-branch", () => ({
  useBranch: () => ({ setBranch, busy: false }),
}));

vi.mock("@/features/services/hooks/use-build-filter", () => ({
  useBuildFilter: () => ({ setBuildFilter, busy: false }),
}));

vi.mock("@/features/services/hooks/use-build-command", () => ({
  useBuildCommand: () => ({ setBuildCommand, busy: false }),
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

// The Deploy card embeds the Deploy Hook rows (w5/m52), which read useDeployHook.
vi.mock("@/features/services/hooks/use-deploy-hook", () => ({
  useDeployHook: () => ({
    url: "https://api.bex.co/v1/deploy-hooks/dhk-test",
    loading: false,
    error: undefined,
    regenerate: vi.fn(async () => true),
    regenerating: false,
  }),
}));

// The Branch combobox reads the repo's branches (w5/m54); empty => free-text.
const repoBranches = { branches: [] as string[], loading: false };
vi.mock("@/features/services/hooks/use-repo-branches", () => ({
  useRepoBranches: () => repoBranches,
}));

// The Source combobox (w5/m54) reads useRepos and writes useSetRepo.
const setRepo = vi.fn(async () => true);
vi.mock("@/features/services/hooks/use-set-repo", () => ({
  useSetRepo: () => ({ setRepo, busy: false }),
}));
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({ repos: [], loading: false, error: undefined }),
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
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  setRootDir.mockClear();
  setRootDir.mockResolvedValue(true);
  setBranch.mockClear();
  setBranch.mockResolvedValue(true);
  setAutoDeploy.mockClear();
  setAutoDeploy.mockResolvedValue(true);
  setPreDeployCommand.mockClear();
  setPreDeployCommand.mockResolvedValue(true);
  setBuildFilter.mockClear();
  setBuildFilter.mockResolvedValue(true);
  setBuildCommand.mockClear();
  setBuildCommand.mockResolvedValue(true);
  setStartCommand.mockClear();
  setStartCommand.mockResolvedValue(true);
  setDockerfilePath.mockClear();
  setRepo.mockClear();
  setRepo.mockResolvedValue(true);
  setDockerfilePath.mockResolvedValue(true);
  connectionState.connection = undefined;
});

/** Confirm a rebuild-affecting save through the row's AlertDialog. */
async function confirmSave(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  await user.click(
    within(screen.getByRole("alertdialog")).getByRole("button", {
      name: "Save changes",
    }),
  );
}

describe("BuildDeploySection", () => {
  it("gates the create-only build config for a contributor, but not Auto-Deploy (w9/m84)", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
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
    // A contributor cannot choose what the service runs: the create-gated rows
    // hide their edit pencil and show the role reason instead of a 403-on-save.
    expect(
      screen.queryByRole("button", { name: "Edit Pre-Deploy Command" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getAllByText(/change what this service runs/i).length,
    ).toBeGreaterThan(0);
    // Auto-Deploy is can_operate — a contributor keeps it.
    expect(
      screen.getByRole("button", { name: "Edit Auto-Deploy" }),
    ).toBeInTheDocument();
  });

  it("shows the repo read-only and the current branch inside a disabled input", () => {
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
    const source = screen.getByRole("combobox", { name: "Source" });
    expect(source).toHaveValue("https://github.com/x/mono");
    expect(source).toBeDisabled();
    const branch = screen.getByRole("combobox", { name: "Branch" });
    expect(branch).toHaveValue("main");
    expect(branch).toBeDisabled();
  });

  it("edits the Source repo through the confirm dialog via setRepo (w5/m54)", async () => {
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

    await user.click(screen.getByRole("button", { name: "Edit Source" }));
    const source = screen.getByRole("combobox", { name: "Source" });
    await user.clear(source);
    await user.type(source, "https://github.com/x/next");
    // Switching the repo rebuilds, so it confirms first.
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await user.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: "Save changes",
      }),
    );

    expect(setRepo).toHaveBeenCalledWith("app", "https://github.com/x/next");
  });

  it("edits the branch through the confirm dialog via setBranch (w5/m48)", async () => {
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
    await user.click(screen.getByRole("button", { name: "Edit branch" }));
    const input = screen.getByRole("combobox", { name: "Branch" });
    await user.clear(input);
    await user.type(input, "release");
    // The pencil flow confirms before saving (a branch change redeploys).
    await confirmSave(user);
    expect(setBranch).toHaveBeenCalledWith("app", "release");
  });

  it("shows an empty Root Directory input when rootDir is unset", () => {
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
    expect(screen.getByRole("textbox", { name: "Root Directory" })).toHaveValue(
      "",
    );
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
    expect(screen.getByRole("textbox", { name: "Root Directory" })).toHaveValue(
      "backend",
    );
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
    await user.type(screen.getByPlaceholderText("e.g. backend"), "backend");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText("Change Root Directory to backend?"),
    ).toBeInTheDocument();
    await user.click(
      within(dialog).getByRole("button", { name: "Save changes" }),
    );

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
    await user.clear(screen.getByDisplayValue("backend"));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

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
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
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
    expect(screen.getByRole("textbox", { name: "Root Directory" })).toHaveValue(
      "",
    );
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
    await confirmSave(user);

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
    await confirmSave(user);

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
    await confirmSave(user);

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
    await confirmSave(user);

    expect(screen.getByDisplayValue("docker/Dockerfile.prod")).toBeVisible();
  });

  it("Auto-Deploy is a disabled select; editing to 'On Commit' calls setAutoDeploy(id, true)", async () => {
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

    // Render's disabled select, not a switch (w5/m53).
    const select = screen.getByRole("combobox", { name: "Auto-Deploy" });
    expect(select).toBeDisabled();
    expect(select).toHaveTextContent("Off");

    await user.click(screen.getByRole("button", { name: "Edit Auto-Deploy" }));
    await user.click(screen.getByRole("combobox", { name: "Auto-Deploy" }));
    await user.click(screen.getByRole("option", { name: "On Commit" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(setAutoDeploy).toHaveBeenCalledWith("app", true);
  });

  it("reverts the optimistic Auto-Deploy selection when setAutoDeploy fails", async () => {
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

    const select = screen.getByRole("combobox", { name: "Auto-Deploy" });
    expect(select).toHaveTextContent("On Commit");

    await user.click(screen.getByRole("button", { name: "Edit Auto-Deploy" }));
    await user.click(screen.getByRole("combobox", { name: "Auto-Deploy" }));
    await user.click(screen.getByRole("option", { name: "Off" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(setAutoDeploy).toHaveBeenCalledWith("app", false);
    // A failed save keeps the row in edit mode (like the other rows); the
    // optimistic revert restores "On Commit" when the edit is cancelled.
    expect(
      screen.getByRole("button", { name: "Save changes" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(
      screen.getByRole("combobox", { name: "Auto-Deploy" }),
    ).toHaveTextContent("On Commit");
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
    expect(
      screen.queryByRole("textbox", { name: "Pre-Deploy Command" }),
    ).not.toBeInTheDocument();
  });

  it("shows the current pre-deploy command, or an empty input when unset", () => {
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
    expect(
      screen.getByRole("textbox", { name: "Pre-Deploy Command" }),
    ).toHaveValue("npm run migrate");
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
    expect(
      screen.getByRole("textbox", { name: "Pre-Deploy Command" }),
    ).toHaveValue("");
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
    await user.click(screen.getByRole("button", { name: "Save changes" }));

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

  it("shows the Build Command editor only when showBuildCommand is true (w7/m41)", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/site"
        branch="main"
        rootDir={null}
        buildCommand={null}
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(screen.queryByText("Build Command")).not.toBeInTheDocument();

    rerender(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/site"
        branch="main"
        rootDir={null}
        buildCommand="npm run build"
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
        showBuildCommand
      />,
    );
    expect(screen.getByText("Build Command")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Build Command" })).toHaveValue(
      "npm run build",
    );

    await user.click(
      screen.getByRole("button", { name: "Edit build command" }),
    );
    const input = screen.getByDisplayValue("npm run build");
    await user.clear(input);
    await user.type(input, "yarn build");
    await confirmSave(user);

    expect(setBuildCommand).toHaveBeenCalledWith("app", "yarn build");
  });

  // Root-dir command prompt prefix (w5/m51).
  it("renders a '<rootDir>/ $' prompt on Build, Start, and Pre-Deploy inputs", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir="backend"
        runtime="node"
        buildCommand="yarn build"
        startCommand="npm start"
        preDeployCommand="npm run migrate"
        autoDeploy={false}
        showPreDeployCommand={true}
        showStartCommand
        showBuildCommand
      />,
    );
    // Build, Start, and Pre-Deploy each carry the same root-relative prompt.
    expect(screen.getAllByText("backend/ $")).toHaveLength(3);
  });

  it("falls back to a bare '$' prompt when no root dir is set", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir={null}
        runtime="node"
        buildCommand="yarn build"
        startCommand="npm start"
        preDeployCommand={null}
        autoDeploy={false}
        showPreDeployCommand={false}
        showStartCommand
        showBuildCommand
      />,
    );
    // Build + Start both show the bare prompt; none show a rootDir prefix.
    expect(screen.getAllByText("$")).toHaveLength(2);
  });

  it("shows no prompt on the Docker Command (a container CMD, not a rootDir shell command)", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        repo="https://github.com/x/mono"
        branch="main"
        rootDir="backend"
        runtime="docker"
        startCommand="bin/run"
        preDeployCommand={null}
        autoDeploy={false}
        showPreDeployCommand={false}
        showStartCommand
      />,
    );
    expect(screen.getByText("Docker Command")).toBeInTheDocument();
    expect(screen.queryByText("backend/ $")).not.toBeInTheDocument();
  });
});
