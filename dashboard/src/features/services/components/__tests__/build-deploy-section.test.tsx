import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BuildDeploySection } from "@/features/services/components/build-deploy-section";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

const setRootDir = vi.fn(async () => true);
const setAutoDeployTrigger = vi.fn(async () => true);
const setPreDeployCommand = vi.fn(async () => true);
const setBuildFilter = vi.fn(async () => true);
const setBuildCommand = vi.fn(async () => true);
const setStartCommand = vi.fn(async () => true);
const setDockerfilePath = vi.fn(async () => true);
const setDockerContext = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-root-dir", () => ({
  useRootDir: () => ({ setRootDir, busy: false }),
}));

vi.mock("@/features/services/hooks/use-build-filter", () => ({
  useBuildFilter: () => ({ setBuildFilter, busy: false }),
}));

vi.mock("@/features/services/hooks/use-build-command", () => ({
  useBuildCommand: () => ({ setBuildCommand, busy: false }),
}));

const startCommandToasts = vi.hoisted(() => [] as unknown[]);
vi.mock("@/features/services/hooks/use-start-command", () => ({
  useStartCommand: (toasts?: unknown) => {
    startCommandToasts.push(toasts);
    return { setStartCommand, busy: false };
  },
}));

vi.mock("@/features/services/hooks/use-dockerfile-path", () => ({
  useDockerfilePath: () => ({ setDockerfilePath, busy: false }),
}));

vi.mock("@/features/services/hooks/use-docker-context", () => ({
  useDockerContext: () => ({ setDockerContext, busy: false }),
}));

vi.mock("@/features/services/hooks/use-pre-deploy-command", () => ({
  usePreDeployCommand: () => ({ setPreDeployCommand, busy: false }),
}));

vi.mock("@/features/services/hooks/use-auto-deploy", () => ({
  useAutoDeploy: () => ({ setAutoDeployTrigger, busy: false }),
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

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  setRootDir.mockClear();
  setRootDir.mockResolvedValue(true);
  setAutoDeployTrigger.mockClear();
  setAutoDeployTrigger.mockResolvedValue(true);
  setPreDeployCommand.mockClear();
  setPreDeployCommand.mockResolvedValue(true);
  setBuildFilter.mockClear();
  setBuildFilter.mockResolvedValue(true);
  setBuildCommand.mockClear();
  setBuildCommand.mockResolvedValue(true);
  setStartCommand.mockClear();
  setStartCommand.mockResolvedValue(true);
  setDockerfilePath.mockClear();
  setDockerfilePath.mockResolvedValue(true);
  setDockerContext.mockClear();
  setDockerContext.mockResolvedValue(true);
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
  // w4/548: a change's confirmation promises the current version keeps
  // serving only when a release is live; with nothing live yet it says the
  // change goes live with the next successful deploy or build.
  const LIVE_COPY = {
    root: "in-flight requests finish before old instances are replaced",
    build:
      "Existing requests will be drained before old instances are replaced.",
    staticBuild: "Your site keeps serving its current published version",
    start: "In-flight requests finish before old instances are replaced.",
  };
  const NEVER_LIVE =
    "Nothing is serving yet — it goes live when a deploy succeeds.";
  const NEVER_PUBLISHED =
    "Your site isn't published yet — it goes live when this build succeeds.";
  it.each([
    ["Root Directory", false, false, LIVE_COPY.root],
    ["Root Directory", false, true, NEVER_LIVE],
    [
      "Root Directory",
      true,
      false,
      "Your site keeps serving its current published version",
    ],
    ["Root Directory", true, true, NEVER_PUBLISHED],
    ["Build Command", false, false, LIVE_COPY.build],
    ["Build Command", false, true, NEVER_LIVE],
    ["Build Command", true, false, LIVE_COPY.staticBuild],
    ["Build Command", true, true, NEVER_PUBLISHED],
    ["Start Command", false, false, LIVE_COPY.start],
    ["Start Command", false, true, NEVER_LIVE],
  ] as const)(
    "confirms a %s change (static %s, never live %s) truthfully",
    async (field, staticSite, neverLive, want) => {
      const user = userEvent.setup();
      render(
        <BuildDeploySection
          serviceId="app"
          rootDir="web"
          buildCommand="make"
          startCommand="./serve"
          autoDeploy={false}
          preDeployCommand={null}
          showPreDeployCommand={false}
          showBuildCommand
          showStartCommand={!staticSite}
          staticSite={staticSite}
          live={!neverLive}
        />,
      );
      await user.click(
        screen.getByRole("button", {
          name: new RegExp(`^Edit ${field}$`, "i"),
        }),
      );
      const input = screen.getByDisplayValue(
        {
          "Root Directory": "web",
          "Build Command": "make",
          "Start Command": "./serve",
        }[field],
      );
      await user.type(input, "2");
      await user.click(screen.getByRole("button", { name: "Save changes" }));
      const dialog = await screen.findByRole("alertdialog");
      expect(dialog).toHaveTextContent(want);
    },
  );

  it.each([
    ["a Free service with no command", true, null, false],
    ["a Free service with a legacy command", true, "npm run migrate", true],
    ["a paid service", false, null, true],
  ])(
    "gates the Pre-Deploy Command on the plan for %s (w8/m131)",
    (_name, freePlan, command, editable) => {
      render(
        <BuildDeploySection
          serviceId="app"
          rootDir={null}
          autoDeploy={false}
          preDeployCommand={command}
          showPreDeployCommand={true}
          freePlan={freePlan}
        />,
      );
      const edit = screen.queryByRole("button", {
        name: "Edit Pre-Deploy Command",
      });
      if (editable) {
        expect(edit).toBeInTheDocument();
      } else {
        expect(edit).not.toBeInTheDocument();
        expect(
          screen.getByText("Pre-deploy commands are available on paid plans."),
        ).toBeInTheDocument();
      }
    },
  );

  it("gates the create-only build config for a contributor, but not Auto-Deploy (w9/m84)", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    render(
      <BuildDeploySection
        serviceId="app"
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
      screen.getAllByText(/can’t make this change/i).length,
    ).toBeGreaterThan(0);
    // Auto-Deploy is can_operate — a contributor keeps it.
    expect(
      screen.getByRole("button", { name: "Edit Auto-Deploy" }),
    ).toBeInTheDocument();
  });

  it("shows an empty Root Directory input when rootDir is unset", () => {
    render(
      <BuildDeploySection
        serviceId="app"
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

  it("shows the Docker Build Context Directory only for Dockerfile builds and persists an edit (w8/m128)", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <BuildDeploySection
        serviceId="app"
        rootDir="backend"
        runtime="node"
        dockerContext="stale"
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(
      screen.queryByText("Docker Build Context Directory"),
    ).not.toBeInTheDocument();

    rerender(
      <BuildDeploySection
        serviceId="app"
        rootDir="backend"
        runtime="docker"
        dockerfilePath="./Dockerfile"
        dockerContext="."
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
        showDockerfilePath={false}
      />,
    );
    expect(
      screen.queryByText("Docker Build Context Directory"),
    ).not.toBeInTheDocument();

    rerender(
      <BuildDeploySection
        serviceId="app"
        rootDir="backend"
        runtime="docker"
        dockerfilePath="./Dockerfile"
        dockerContext="."
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    // Read back as the server reports it: "." is the Root Directory itself.
    expect(
      screen.getByRole("textbox", { name: "Docker Build Context Directory" }),
    ).toHaveValue(".");

    await user.click(
      screen.getByRole("button", {
        name: "Edit Docker Build Context Directory",
      }),
    );
    const input = screen.getByRole("textbox", {
      name: "Docker Build Context Directory",
    });
    await user.clear(input);
    await user.type(input, "ctx");
    await confirmSave(user);

    expect(setDockerContext).toHaveBeenCalledWith("app", "ctx");
    expect(setDockerfilePath).not.toHaveBeenCalled();
  });

  it("keeps the Dockerfile Path draft after a failed mutation", async () => {
    setDockerfilePath.mockResolvedValue(false);
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
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

  it("Auto-Deploy is a disabled select; editing to 'On Commit' calls setAutoDeployTrigger(id, 'commit')", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
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

    expect(setAutoDeployTrigger).toHaveBeenCalledWith("app", "commit");
  });

  it("offers Render's three Auto-Deploy choices and saves After CI Checks Pass", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        rootDir={null}
        autoDeploy={true}
        autoDeployTrigger="commit"
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit Auto-Deploy" }));
    await user.click(screen.getByRole("combobox", { name: "Auto-Deploy" }));
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["On Commit", "After CI Checks Pass", "Off"]);
    await user.click(
      screen.getByRole("option", { name: "After CI Checks Pass" }),
    );
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(setAutoDeployTrigger).toHaveBeenCalledWith("app", "checksPass");
  });

  it("shows a saved checksPass service as After CI Checks Pass", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        rootDir={null}
        autoDeploy={true}
        autoDeployTrigger="checksPass"
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(
      screen.getByRole("combobox", { name: "Auto-Deploy" }),
    ).toHaveTextContent("After CI Checks Pass");
  });

  it("keeps the saved Auto-Deploy value when setAutoDeployTrigger fails", async () => {
    setAutoDeployTrigger.mockResolvedValue(false);
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
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

    expect(setAutoDeployTrigger).toHaveBeenCalledWith("app", "off");
    // A failed save keeps the row in edit mode with the draft for a retry (like
    // the other rows); Cancel shows the unchanged saved "On Commit".
    expect(
      screen.getByRole("button", { name: "Save changes" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(
      screen.getByRole("combobox", { name: "Auto-Deploy" }),
    ).toHaveTextContent("On Commit");
  });

  // Auto-Deploy push-source hint (w2/m9, corrected w6/m99). The hint states a
  // MECHANISM, so it may only ever come from the server's per-repo grant answer
  // (pushDeliveryMethod) — never from "the workspace has a connection and the
  // URL says github.com", which promised the GitHub app for repos the
  // installation does not grant and where no push ever arrives. The prop is the
  // whole input: workspace connection state and the picker's granted-repo list
  // reach the hint only through the server's answer, so neither is mocked here.

  // The w2/m9 control case, now asserted rather than assumed: a repo the
  // installation really grants keeps the GitHub-app claim.
  it("names the GitHub app as the push source for a repo the installation grants", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        rootDir={null}
        autoDeploy={true}
        pushDeliveryMethod="github_app"
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(
      screen.getByText(/redeploys automatically via the GitHub app/),
    ).toBeInTheDocument();
  });

  // The live-found regression (w6/m99): connection is live AND the repo is a
  // github.com URL — the two conditions the old heuristic checked — but the
  // installation does not grant this repo, so GitHub never delivers a push
  // event for it. The old code claimed the GitHub app here.
  it("names the manual webhook for a github.com repo the live connection does not grant", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        rootDir={null}
        autoDeploy={true}
        pushDeliveryMethod="manual_webhook"
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(screen.getByText(/manual git webhook/)).toBeInTheDocument();
    expect(screen.queryByText(/via the GitHub app/)).not.toBeInTheDocument();
  });

  it("names the manual webhook as the push source when GitHub is not connected", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        rootDir={null}
        autoDeploy={true}
        pushDeliveryMethod="manual_webhook"
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(screen.getByText(/manual git webhook/)).toBeInTheDocument();
  });

  // w4/483: a multitenant install rejects the shared manual webhook key, so a
  // repo the GitHub App does not cover must not promise a webhook.
  it("says auto-deploy can't run when no push can reach bex", () => {
    render(
      <BuildDeploySection
        serviceId="app"
        rootDir={null}
        autoDeploy={true}
        pushDeliveryMethod="unavailable"
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(
      screen.getByText(/Auto-deploy can't run for this repo/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/manual git webhook/)).not.toBeInTheDocument();
    expect(screen.queryByText(/BEX_WEBHOOK_SECRET/)).not.toBeInTheDocument();
  });

  // GitHub unreachable, and the not-yet-loaded first render: name no mechanism
  // at all rather than pick one and be wrong in whichever direction.
  it.each([
    ["unknown", "unknown" as string | undefined],
    ["not yet loaded", undefined],
  ])("states the uncertainty when delivery is %s", (_label, method) => {
    render(
      <BuildDeploySection
        serviceId="app"
        rootDir={null}
        autoDeploy={true}
        pushDeliveryMethod={method}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(
      screen.getByText(/Checking how a push to the tracked branch reaches bex/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/via the GitHub app/)).not.toBeInTheDocument();
    expect(screen.queryByText(/manual git webhook/)).not.toBeInTheDocument();
  });

  // Pre-Deploy Command field (w1/m33).
  it("hides the Pre-Deploy Command field when showPreDeployCommand is false", () => {
    render(
      <BuildDeploySection
        serviceId="app"
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

  it("confirms the deploy, then saves the pre-deploy command via setPreDeployCommand (w4/616)", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
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

    // The save starts a deploy that runs the new command before it goes live.
    const dialog = screen.getByRole("alertdialog");
    expect(
      within(dialog).getByText(/Change the Pre-Deploy Command to/),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(/the new command runs before the new release/),
    ).toBeInTheDocument();
    expect(setPreDeployCommand).not.toHaveBeenCalled();
    await user.click(
      within(dialog).getByRole("button", { name: "Save changes" }),
    );

    // A nonblank command reaches the mutation as typed, padding included
    // (w8/m68).
    expect(setPreDeployCommand).toHaveBeenCalledWith(
      "app",
      "  npm run migrate  ",
    );
  });

  // Build Filters editor (w1/m34).
  it("renders the Build Filters editor with both path lists", () => {
    render(
      <BuildDeploySection
        serviceId="app"
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

  it("disables Build Filters for a viewer without can_operate", () => {
    // SetBuildFilter is RelCanOperate, not can_create — a contributor still edits.
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        role: "VIEWER",
        canCreate: false,
        canOperate: false,
      }),
    );
    render(
      <BuildDeploySection
        serviceId="app"
        rootDir={null}
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Add included path" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Add ignored path" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Save Build Filters" }),
    ).toBeDisabled();
    expect(
      screen.getByText(/Your role can only view this service/i),
    ).toBeInTheDocument();
  });

  it("lets a contributor save Build Filters (can_operate)", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
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

  it("shows the Build Command editor only when showBuildCommand is true (w7/m41)", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <BuildDeploySection
        serviceId="app"
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

  it("tells a static site its published version keeps serving; Cancel saves nothing (w4/246)", async () => {
    const user = userEvent.setup();
    render(
      <BuildDeploySection
        serviceId="app"
        rootDir={null}
        buildCommand="npm run build"
        autoDeploy={false}
        preDeployCommand={null}
        showPreDeployCommand={false}
        showBuildCommand
        staticSite
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit build command" }),
    );
    const input = screen.getByDisplayValue("npm run build");
    await user.clear(input);
    await user.type(input, "yarn build");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    const dialog = screen.getByRole("alertdialog");
    expect(
      within(dialog).getByText(/keeps serving its current published version/),
    ).toBeInTheDocument();
    expect(within(dialog).queryByText(/drained/)).not.toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(setBuildCommand).not.toHaveBeenCalled();

    await confirmSave(user);
    expect(setBuildCommand).toHaveBeenCalledTimes(1);
    expect(setBuildCommand).toHaveBeenCalledWith("app", "yarn build");
  });

  // Root-dir command prompt prefix (w5/m51).
  it("renders a '<rootDir>/ $' prompt on Build, Start, and Pre-Deploy inputs", () => {
    render(
      <BuildDeploySection
        serviceId="app"
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

  // w4/357: a Docker Command save toasted "Start Command updated.", naming a
  // field the card never showed. Each kind passes the toasts for its own label.
  it.each([
    ["node", "services.startCommandSuccess", "services.startCommandError"],
    ["docker", "services.dockerCommandSuccess", "services.dockerCommandError"],
  ])(
    "a %s service's command save toasts its own field",
    (runtime, success, error) => {
      startCommandToasts.length = 0;
      render(
        <BuildDeploySection
          serviceId="app"
          rootDir={null}
          runtime={runtime}
          startCommand="bin/run"
          preDeployCommand={null}
          autoDeploy={false}
          showPreDeployCommand={false}
          showStartCommand
        />,
      );
      expect(startCommandToasts.at(-1)).toEqual({ success, error });
    },
  );

  it("shows no prompt on the Docker Command (a container CMD, not a rootDir shell command)", () => {
    render(
      <BuildDeploySection
        serviceId="app"
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

// w4/281: the Auto-Deploy row kept its own useState copy of the saved value, so
// a poll (or another tab's save) reaching this mounted section changed nothing
// until a full reload — the untouched row, its Edit baseline and Cancel all
// showed the stale value.
describe("Auto-Deploy — saved value refresh (w4/281)", () => {
  const section = (autoDeploy: boolean, rootDir: string | null = null) => (
    <BuildDeploySection
      serviceId="app"
      rootDir={rootDir}
      autoDeploy={autoDeploy}
      preDeployCommand={null}
      showPreDeployCommand={false}
    />
  );
  const select = () => screen.getByRole("combobox", { name: "Auto-Deploy" });
  const save = () => screen.getByRole("button", { name: "Save changes" });

  it("an untouched row follows newer saved values in both directions", async () => {
    const user = userEvent.setup();
    const view = render(section(false));
    expect(select()).toHaveTextContent("Off");

    view.rerender(section(true));
    expect(select()).toHaveTextContent("On Commit");

    // Edit starts from the newer value: re-choosing it is not a change.
    await user.click(screen.getByRole("button", { name: "Edit Auto-Deploy" }));
    expect(select()).toHaveTextContent("On Commit");
    expect(save()).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(select()).toHaveTextContent("On Commit");

    view.rerender(section(false));
    expect(select()).toHaveTextContent("Off");
    await user.click(screen.getByRole("button", { name: "Edit Auto-Deploy" }));
    expect(save()).toBeDisabled();
    await user.click(select());
    await user.click(screen.getByRole("option", { name: "On Commit" }));
    expect(save()).toBeEnabled();
  });

  it("keeps an open draft across a saved-value read, and Cancel returns to the latest value", async () => {
    const user = userEvent.setup();
    const view = render(section(false));

    await user.click(screen.getByRole("button", { name: "Edit Auto-Deploy" }));
    await user.click(select());
    await user.click(screen.getByRole("option", { name: "On Commit" }));

    // An unrelated read leaves the draft alone.
    view.rerender(section(false, "apps/web"));
    expect(select()).toHaveTextContent("On Commit");
    expect(save()).toBeEnabled();

    // Another tab saved On Commit: the draft now matches the stored value.
    view.rerender(section(true, "apps/web"));
    expect(select()).toHaveTextContent("On Commit");
    expect(save()).toBeDisabled();

    // Cancel shows the latest saved value, not the one the edit began from.
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(select()).toHaveTextContent("On Commit");
  });
});

// w4/274: an untouched Build Filters editor kept its mount-time lists after a
// newer saved filter arrived (another tab), so it offered them as an unsaved
// change whose Save rolled the newer filter back.
describe("BuildFilterEditor — saved filter refresh (w4/274)", () => {
  type Filter = { paths: string[]; ignoredPaths: string[] } | null;
  const section = (buildFilter: Filter) => (
    <BuildDeploySection
      serviceId="app"
      rootDir={null}
      buildFilter={buildFilter}
      autoDeploy={false}
      preDeployCommand={null}
      showPreDeployCommand={false}
    />
  );
  const save = () => screen.getByRole("button", { name: "Save Build Filters" });
  const cancel = () => screen.queryByRole("button", { name: "Cancel" });
  const included = () =>
    screen
      .queryAllByPlaceholderText("e.g. src/**")
      .map((input) => (input as HTMLInputElement).value);
  const ignored = () =>
    screen
      .queryAllByPlaceholderText("e.g. docs/**")
      .map((input) => (input as HTMLInputElement).value);

  it("an untouched draft adopts a newer saved filter and stays clean", () => {
    const view = render(
      section({ paths: ["site/**", "docs/a b.md"], ignoredPaths: ["**/*.md"] }),
    );

    view.rerender(
      section({
        paths: ["site/index.html", "docs/a b.md"],
        ignoredPaths: ["**/*.md"],
      }),
    );
    expect(included()).toEqual(["site/index.html", "docs/a b.md"]);
    expect(save()).toBeDisabled();
    expect(cancel()).toBeNull();

    view.rerender(
      section({ paths: ["site/index.html"], ignoredPaths: ["docs/**"] }),
    );
    expect(ignored()).toEqual(["docs/**"]);
    expect(save()).toBeDisabled();

    view.rerender(section(null));
    expect(included()).toEqual([]);
    expect(ignored()).toEqual([]);
    expect(save()).toBeDisabled();

    view.rerender(section({ paths: ["src/**"], ignoredPaths: [] }));
    expect(included()).toEqual(["src/**"]);
    expect(save()).toBeDisabled();
  });

  it("a real edit survives refreshes, keeps focus, and Cancel adopts the latest", async () => {
    const user = userEvent.setup();
    const view = render(
      section({ paths: ["site/**"], ignoredPaths: ["**/*.md"] }),
    );
    const input = screen.getByDisplayValue("site/**");
    await user.clear(input);
    await user.type(input, "qa-unsaved/**");

    view.rerender(section({ paths: ["site/**"], ignoredPaths: ["**/*.md"] }));
    view.rerender(section(null));
    expect(included()).toEqual(["qa-unsaved/**"]);
    expect(ignored()).toEqual(["**/*.md"]);
    expect(input).toHaveFocus();
    expect(save()).toBeEnabled();

    await user.click(cancel()!);
    expect(included()).toEqual([]);
    expect(ignored()).toEqual([]);
    expect(save()).toBeDisabled();
  });

  it("keeps typing done while a save is pending, adopting the saved lists otherwise", async () => {
    const user = userEvent.setup();
    let settle: (ok: boolean) => void = () => {};
    setBuildFilter.mockImplementationOnce(
      () => new Promise<boolean>((resolve) => (settle = resolve)),
    );
    const view = render(section({ paths: ["site/**"], ignoredPaths: [] }));
    const input = screen.getByDisplayValue("site/**");
    await user.type(input, "  ");
    await user.click(save());
    expect(setBuildFilter).toHaveBeenCalledWith("app", ["site/**"], []);

    await user.type(input, "x");
    view.rerender(section({ paths: ["site/**"], ignoredPaths: [] }));
    await act(async () => settle(true));
    expect(included()).toEqual(["site/**  x"]);
    expect(save()).toBeEnabled();
  });

  it("adopts the normalized lists after an accepted save with no later typing", async () => {
    const user = userEvent.setup();
    const view = render(section({ paths: ["site/**"], ignoredPaths: [] }));
    await user.type(screen.getByDisplayValue("site/**"), "  ");
    await user.click(save());
    view.rerender(section({ paths: ["site/**"], ignoredPaths: [] }));
    expect(included()).toEqual(["site/**"]);
    expect(save()).toBeDisabled();
  });
});

// w21/045: a build setting the server would refuse is marked in its row, with
// Save held and no "the service rebuilds and redeploys" confirmation, instead of
// a toast naming the wire field after the round trip.
describe("build setting row validation (w21/045)", () => {
  const section = (
    props: Partial<Parameters<typeof BuildDeploySection>[0]>,
  ) => (
    <BuildDeploySection
      serviceId="app"
      rootDir="backend"
      runtime="docker"
      startCommand={null}
      dockerfilePath="Dockerfile"
      dockerContext="."
      autoDeploy={false}
      preDeployCommand={null}
      showPreDeployCommand={false}
      showStartCommand
      {...props}
    />
  );

  async function editRow(
    user: ReturnType<typeof userEvent.setup>,
    label: string,
    value: string,
  ) {
    await user.click(screen.getByRole("button", { name: `Edit ${label}` }));
    const input = screen.getByRole("textbox", { name: label });
    await user.clear(input);
    await user.type(input, value);
    return input;
  }

  it.each([
    [
      "Root Directory",
      "../x",
      "Must stay inside the repository: no .. segment",
    ],
    [
      "Dockerfile Path",
      "../Dockerfile",
      "Must stay inside the repository: no .. segment",
    ],
    [
      "Docker Build Context Directory",
      "../ctx",
      "Must stay inside the repository: no .. segment",
    ],
    ["Root Directory", "src\\app", "Use / as the path separator, not \\"],
    [
      "Dockerfile Path",
      "/Dockerfile",
      "Must be relative to the repository, not start with /",
    ],
    [
      "Dockerfile Path",
      "docker//Dockerfile",
      "Must not have an empty segment: no // and no trailing /",
    ],
  ])("marks %s %j and holds Save", async (label, value, message) => {
    const user = userEvent.setup();
    render(section({}));
    const input = await editRow(user, label, value);

    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAccessibleDescription(message);
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    // The rebuild confirmation never opens for a value that can never build.
    await user.type(input, "{Enter}");
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(setRootDir).not.toHaveBeenCalled();
    expect(setDockerfilePath).not.toHaveBeenCalled();
    expect(setDockerContext).not.toHaveBeenCalled();
  });

  it("marks a 513-byte directory with its own reason", async () => {
    const user = userEvent.setup();
    render(section({}));
    const input = screen.getByRole("button", { name: "Edit Root Directory" });
    await user.click(input);
    fireEvent.change(screen.getByRole("textbox", { name: "Root Directory" }), {
      target: { value: "a".repeat(513) },
    });
    expect(
      screen.getByRole("textbox", { name: "Root Directory" }),
    ).toHaveAccessibleDescription("At most 512 bytes");
  });

  it.each(["./", "server/", "a//b"])(
    "still saves the Root Directory spelling %j, which the server normalizes",
    async (value) => {
      const user = userEvent.setup();
      render(section({}));
      await editRow(user, "Root Directory", value);
      expect(
        screen.getByRole("textbox", { name: "Root Directory" }),
      ).not.toHaveAttribute("aria-invalid");
      await confirmSave(user);
      expect(setRootDir).toHaveBeenCalledWith("app", value);
    },
  );

  it.each(["./", "backend/", "./backend/"])(
    "still saves the Docker Build Context spelling %j, which the server normalizes (w8/292)",
    async (value) => {
      const user = userEvent.setup();
      render(section({}));
      await editRow(user, "Docker Build Context Directory", value);
      expect(
        screen.getByRole("textbox", { name: "Docker Build Context Directory" }),
      ).not.toHaveAttribute("aria-invalid");
      await confirmSave(user);
      expect(setDockerContext).toHaveBeenCalledWith("app", value);
    },
  );

  it("marks an unclosed [ in a Build Filter pattern and holds its Save", async () => {
    const user = userEvent.setup();
    render(section({}));
    await user.click(screen.getByRole("button", { name: "Add included path" }));
    const input = screen.getByPlaceholderText("e.g. src/**");
    // user-event reads a lone [ as a key descriptor; [[ types the character.
    await user.type(input, "[[");

    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAccessibleDescription("A [ is never closed by a ]");
    expect(
      screen.getByRole("button", { name: "Save Build Filters" }),
    ).toBeDisabled();
    expect(setBuildFilter).not.toHaveBeenCalled();

    await user.clear(input);
    await user.type(input, "src/**");
    expect(input).not.toHaveAttribute("aria-invalid");
    await user.click(
      screen.getByRole("button", { name: "Save Build Filters" }),
    );
    expect(setBuildFilter).toHaveBeenCalledWith("app", ["src/**"], []);
  });
});
