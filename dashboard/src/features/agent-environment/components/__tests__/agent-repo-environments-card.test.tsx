import { beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { AgentRepoEnvironmentsCard } from "@/features/agent-environment/components/agent-repo-environments-card";
import type { UseAgentRepoEnvironmentsResult } from "@/features/agent-environment/hooks/use-agent-repo-environments";
import type { RepoEnvironmentView } from "@/features/agent-environment/lib/repo-environments";

const hook = vi.hoisted(() => ({
  current: null as unknown as UseAgentRepoEnvironmentsResult,
}));

vi.mock(
  "@/features/agent-environment/hooks/use-agent-repo-environments",
  () => ({ useAgentRepoEnvironments: () => hook.current }),
);

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

// The add dialog's repository picker offers the workspace's connected
// repositories, as the secrets card's repo scope does.
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({
    repos: [{ fullName: "acme/api" }, { fullName: "acme/web" }],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));

const API: RepoEnvironmentView = {
  repo: "acme/api",
  setup: [
    { name: "deps", run: "npm ci" },
    { name: "build", run: "npm run build" },
  ],
  checks: [
    { name: "lint", run: "npm run lint" },
    { name: "test", run: "npm test" },
  ],
};

function setup(
  environments: RepoEnvironmentView[] = [API],
  over: Partial<UseAgentRepoEnvironmentsResult> = {},
) {
  const set = vi.fn(async () => undefined);
  const remove = vi.fn(async () => undefined);
  hook.current = {
    environments,
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
    set,
    remove,
    saving: false,
    ...over,
  };
  render(<AgentRepoEnvironmentsCard ownerId="tea-1" />);
  return { set, remove, user: userEvent.setup() };
}

const dialog = () => screen.getByTestId("agent-repo-environment-dialog");
const field = (label: string) => within(dialog()).getByLabelText(label);
const saveButton = () => within(dialog()).getByRole("button", { name: "Save" });

async function openAdd(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Add environment" }));
}

describe("AgentRepoEnvironmentsCard (ADR109)", () => {
  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    vi.mocked(toast.success).mockClear();
  });

  it("renders the skeleton, with the static header, while loading", () => {
    setup([], { loading: true });
    const skeleton = screen.getByTestId("agent-repo-environments-skeleton");
    expect(skeleton).toHaveTextContent("Repository environments");
    expect(skeleton.querySelector("[data-slot=card-title]")).not.toBeNull();
    expect(
      skeleton.querySelector("[data-slot=card-description]"),
    ).not.toBeNull();
  });

  // The skeleton previews the ready card region for region: one 3.5rem
  // repository row (the empty line has the same height), then the Add button
  // and the help line reserved at its taller variant (dashboard/AGENTS.md).
  it("draws the ready card's regions at their heights", () => {
    setup([], { loading: true });
    const skeleton = screen.getByTestId("agent-repo-environments-skeleton");
    const [skeletonRow, skeletonActions] = Array.from(
      skeleton.querySelector("[data-slot=card-content]")!.children,
    );
    // 3.5rem inside a 1px border: the list's box around one row.
    const rowBox = ["box-content", "h-14", "border", "border-transparent"];
    expect(skeletonRow).toHaveClass(...rowBox);
    expect(skeletonActions).toHaveClass("space-y-2", "border-t", "pt-4");
    // The Add bar is the button's own box: its height, padding, icon and
    // label, so it is the button's width in every language.
    const addBar = screen.getByTestId("repo-envs-add-skeleton");
    expect(addBar).toHaveClass(
      "h-9",
      "border",
      "px-3",
      "text-sm",
      "font-medium",
    );
    expect(addBar).toHaveTextContent("Add environment");
    expect(addBar.querySelector(".size-4")).not.toBeNull();
    const help = screen.getByTestId("repo-envs-help-skeleton");
    expect(help).toHaveClass("grid", "text-xs");
    const [admin, member] = Array.from(help.children);
    expect(admin).toHaveClass("col-start-1", "row-start-1");
    expect(member).toHaveClass("invisible", "col-start-1", "row-start-1");
    expect(admin.textContent).not.toBe("");
    expect(member.textContent).not.toBe("");
    cleanup();

    setup([]);
    const empty = screen.getByText("No repository environments yet.");
    expect(empty).toHaveClass(...rowBox);
    const addButton = screen.getByRole("button", { name: "Add environment" });
    expect(addButton).toHaveClass("h-9", "border", "has-[>svg]:px-3");
    expect(addButton.closest(".border-t")).toHaveClass("space-y-2", "pt-4");
    cleanup();

    setup([API]);
    const list = screen.getByTestId("agent-repo-environments");
    expect(list).toHaveClass("border");
    expect(list.firstChild).toHaveClass("h-14");
  });

  it("lists each repository with its setup steps and checks", async () => {
    const { user } = setup([
      API,
      {
        repo: "acme/web",
        setup: [{ name: "deps", run: "pnpm i" }],
        checks: [],
      },
    ]);
    const summaries = screen
      .getAllByTestId("agent-repo-environment-summary")
      .map((line) => line.textContent);
    expect(summaries).toEqual([
      "Setup: deps, build · Checks: lint, test",
      "Setup: deps · No checks",
    ]);
    expect(screen.getByText("acme/api")).toBeInTheDocument();
    // The editor shows each command, in order.
    await user.click(screen.getByRole("button", { name: "Edit acme/api" }));
    expect(field("Setup step 1 name")).toHaveValue("deps");
    expect(field("Setup step 2 command")).toHaveValue("npm run build");
    expect(field("Check 2 command")).toHaveValue("npm test");
  });

  it("adds an environment and sends it whole", async () => {
    const { user, set } = setup([]);
    await openAdd(user);
    await user.type(
      within(dialog()).getByRole("combobox", { name: "Repository" }),
      "Acme/Worker",
    );
    await user.type(field("Setup step 1 name"), "deps");
    await user.type(field("Setup step 1 command"), "npm ci");
    await user.click(
      within(dialog()).getByRole("button", { name: "Add setup step" }),
    );
    await user.type(field("Setup step 2 name"), "build");
    await user.type(field("Setup step 2 command"), "npm run build");
    await user.type(field("Check 1 name"), "test");
    await user.type(field("Check 1 command"), "npm test");
    await user.click(saveButton());
    expect(set).toHaveBeenCalledWith("acme/worker", {
      setup: [
        { name: "deps", run: "npm ci" },
        { name: "build", run: "npm run build" },
      ],
      checks: [{ name: "test", run: "npm test" }],
    });
    expect(toast.success).toHaveBeenCalledWith(
      "Saved the environment for acme/worker",
    );
    expect(
      screen.queryByTestId("agent-repo-environment-dialog"),
    ).not.toBeInTheDocument();
  });

  // A save replaces the stored environment, so every step goes out, the
  // untouched ones included, in the order the admin left them.
  it("saves an edit as the whole reordered environment", async () => {
    const { user, set } = setup();
    await user.click(screen.getByRole("button", { name: "Edit acme/api" }));
    expect(
      within(dialog()).queryByRole("combobox", { name: "Repository" }),
    ).toBeNull();
    await user.click(
      within(dialog()).getByRole("button", { name: "Move setup step 2 up" }),
    );
    await user.click(
      within(dialog()).getByRole("button", { name: "Remove check 1" }),
    );
    await user.clear(field("Check 1 command"));
    await user.type(field("Check 1 command"), "npm test -- --ci");
    await user.click(saveButton());
    expect(set).toHaveBeenCalledWith("acme/api", {
      setup: [
        { name: "build", run: "npm run build" },
        { name: "deps", run: "npm ci" },
      ],
      checks: [{ name: "test", run: "npm test -- --ci" }],
    });
  });

  // The empty rows a new environment opens with are dropped, not refused.
  it("drops blank rows", async () => {
    const { user, set } = setup([]);
    await openAdd(user);
    await user.type(
      within(dialog()).getByRole("combobox", { name: "Repository" }),
      "acme/web",
    );
    await user.type(field("Check 1 name"), "lint");
    await user.type(field("Check 1 command"), "  npm run lint  ");
    await user.click(saveButton());
    expect(set).toHaveBeenCalledWith("acme/web", {
      setup: [],
      checks: [{ name: "lint", run: "npm run lint" }],
    });
  });

  it("deletes after confirmation", async () => {
    const { user, remove } = setup();
    await user.click(
      screen.getByRole("button", {
        name: "Delete the environment for acme/api",
      }),
    );
    await user.click(
      screen.getByRole("button", { name: "Delete environment" }),
    );
    expect(remove).toHaveBeenCalledWith("acme/api");
    expect(toast.success).toHaveBeenCalledWith(
      "Deleted the environment for acme/api",
    );
  });

  it("refuses malformed and repeated names before saving", async () => {
    const { user, set } = setup([]);
    await openAdd(user);
    await user.type(
      within(dialog()).getByRole("combobox", { name: "Repository" }),
      "acme/web",
    );
    await user.type(field("Setup step 1 name"), "-deps");
    expect(field("Setup step 1 name")).toHaveAttribute("aria-invalid", "true");
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(
      "A name starts with a letter or digit",
    );
    await user.clear(field("Setup step 1 name"));
    await user.type(field("Setup step 1 name"), "deps");
    await user.type(field("Setup step 1 command"), "npm ci");
    await user.type(field("Check 1 name"), "test");
    await user.type(field("Check 1 command"), "npm test");
    await user.click(
      within(dialog()).getByRole("button", { name: "Add check" }),
    );
    await user.type(field("Check 2 name"), "test");
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(
      "Another row in this list already uses this name.",
    );
    await user.click(saveButton());
    expect(set).not.toHaveBeenCalled();
  });

  it("refuses a check command over 2 KiB", async () => {
    const { user, set } = setup([]);
    await openAdd(user);
    await user.type(
      within(dialog()).getByRole("combobox", { name: "Repository" }),
      "acme/web",
    );
    await user.type(field("Check 1 name"), "test");
    await user.click(field("Check 1 command"));
    await user.paste("x".repeat(2 * 1024 + 1));
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(
      "A check command can be at most 2 KiB.",
    );
    await user.click(saveButton());
    expect(set).not.toHaveBeenCalled();
  });

  // A blank required field is flagged only once Save is tried.
  it("flags a missing command on save", async () => {
    const { user, set } = setup([]);
    await openAdd(user);
    await user.type(
      within(dialog()).getByRole("combobox", { name: "Repository" }),
      "acme/web",
    );
    await user.type(field("Setup step 1 name"), "deps");
    expect(within(dialog()).queryByText("Enter a command.")).toBeNull();
    await user.click(saveButton());
    expect(within(dialog()).getByText("Enter a command.")).toBeInTheDocument();
    expect(field("Setup step 1 command")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(set).not.toHaveBeenCalled();
  });

  it("refuses to save an environment with no steps", async () => {
    const { user, set } = setup([]);
    await openAdd(user);
    await user.type(
      within(dialog()).getByRole("combobox", { name: "Repository" }),
      "acme/web",
    );
    await user.click(saveButton());
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(
      "Add at least one setup step or check.",
    );
    expect(set).not.toHaveBeenCalled();
  });

  it("refuses a repository that is not owner/name or already has one", async () => {
    const { user, set } = setup();
    await openAdd(user);
    const repo = within(dialog()).getByRole("combobox", { name: "Repository" });
    await user.type(repo, "acme");
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(
      "Enter the repository as owner/name.",
    );
    await user.type(repo, "/API");
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(
      "This repository already has an environment.",
    );
    await user.type(field("Check 1 name"), "test");
    await user.type(field("Check 1 command"), "npm test");
    await user.click(saveButton());
    expect(set).not.toHaveBeenCalled();
  });

  it("stops adding rows at a list's limit", async () => {
    const { user } = setup([
      {
        repo: "acme/api",
        setup: Array.from({ length: 10 }, (_, i) => ({
          name: `step${i}`,
          run: "true",
        })),
        checks: [],
      },
    ]);
    await user.click(screen.getByRole("button", { name: "Edit acme/api" }));
    expect(
      within(dialog()).getByRole("button", { name: "Add setup step" }),
    ).toBeDisabled();
    expect(
      within(dialog()).getByRole("button", { name: "Add check" }),
    ).toBeEnabled();
  });

  it.each([
    [
      codedGraphQLError("AGENT_REPO_ENVIRONMENT_INVALID", { field: "checks" }),
      "A check was refused.",
    ],
    [
      codedGraphQLError("AGENT_REPO_ENVIRONMENT_INVALID", { field: "setup" }),
      "A setup step was refused.",
    ],
    [
      codedGraphQLError("AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED"),
      "reached its limit of 50 repository environments",
    ],
    [
      codedGraphQLError("AGENT_REPO_ENVIRONMENTS_UNAVAILABLE"),
      "aren't available on this deployment",
    ],
  ])("maps a server refusal %#", async (error, copy) => {
    const { user, set } = setup();
    set.mockRejectedValueOnce(error);
    await user.click(screen.getByRole("button", { name: "Edit acme/api" }));
    await user.click(saveButton());
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(copy);
    // The dialog stays open on the admin's edits.
    expect(field("Setup step 1 name")).toHaveValue("deps");
  });

  it("maps a refused delete", async () => {
    const { user, remove } = setup();
    remove.mockRejectedValueOnce(
      codedGraphQLError("AGENT_REPO_ENVIRONMENT_NOT_FOUND"),
    );
    await user.click(
      screen.getByRole("button", {
        name: "Delete the environment for acme/api",
      }),
    );
    await user.click(
      screen.getByRole("button", { name: "Delete environment" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This repository no longer has an environment",
    );
  });

  it("says when the deployment has no repository environments", () => {
    setup([], {
      error: codedGraphQLError("AGENT_REPO_ENVIRONMENTS_UNAVAILABLE"),
    });
    expect(
      screen.getByText(
        "Repository environments aren't available on this deployment.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("is read-only for a member without can_manage", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    const { user } = setup();
    expect(screen.getByText("acme/api")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Add environment" }),
    ).toBeDisabled();
    expect(
      screen.queryByRole("button", {
        name: "Delete the environment for acme/api",
      }),
    ).toBeNull();
    expect(screen.queryByRole("button", { name: "Edit acme/api" })).toBeNull();
    expect(
      screen.getByText(
        "Only workspace admins can add, change or delete repository environments.",
      ),
    ).toBeInTheDocument();
    // A member can still read every command.
    await user.click(screen.getByRole("button", { name: "View acme/api" }));
    const steps = within(dialog()).getByTestId("repo-environment-setup");
    expect(steps).toHaveTextContent("deps");
    expect(steps).toHaveTextContent("npm run build");
    expect(within(dialog()).queryByRole("textbox")).toBeNull();
    expect(within(dialog()).queryByRole("button", { name: "Save" })).toBeNull();
    expect(
      within(dialog()).getAllByRole("button", { name: "Close" }).length,
    ).toBeGreaterThan(0);
  });
});
