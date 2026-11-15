import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { NewSandboxGroupDialog } from "@/features/sandbox-groups/components/new-sandbox-group-dialog";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { ProjectView } from "@/features/projects/hooks/use-projects";

// The real hook runs; only its two collaborators are stubbed, so these tests
// cover the dialog and the code-to-inline-error mapping together.
const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
}));
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-owner" }),
}));

const copy = vi.fn();
vi.mock("@/common/hooks/use-copy-to-clipboard", () => ({
  useCopyToClipboard: () => ({ copied: false, copy }),
}));

// The environment index the env group dialog reads, stubbed: its own tests
// cover the read. `owners` records who the dialog asked it to read for.
const owners: Array<string | null> = [];
const index = {
  ready: true,
  loading: false,
  error: undefined as Error | undefined,
  projects: [] as ProjectView[],
  environments: [] as EnvironmentView[],
  byId: new Map<string, EnvironmentView>(),
  retry: vi.fn(),
};
vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => ({
  useWorkspaceEnvironmentIndex: (owner: string | null) => {
    owners.push(owner);
    return index;
  },
}));

// A confirmed non-Admin cannot place a group in a protected environment.
const placement = {
  restricts: (environment: { protectedStatus?: string }) =>
    placement.restrictsProtected && environment.protectedStatus === "protected",
  restrictsProtected: false,
  reason: "Only an admin can create resources in a protected environment.",
};
vi.mock("@/features/environments/hooks/use-protected-placement", () => ({
  useProtectedPlacement: () => placement,
}));

const environment = (
  id: string,
  name: string,
  protectedStatus = "unprotected",
) =>
  ({
    id,
    projectId: "prj-shop",
    name,
    protectedStatus,
    serviceIds: [],
    databaseIds: [],
    keyValueIds: [],
    envGroupIds: [],
  }) as unknown as EnvironmentView;

/** The workspace's environments, as the index holds them once read. */
function readEnvironments(...environments: EnvironmentView[]) {
  index.ready = true;
  index.loading = false;
  index.error = undefined;
  index.projects = [{ id: "prj-shop", name: "Shop" } as ProjectView];
  index.environments = environments;
  index.byId = new Map(environments.map((item) => [item.id, item]));
}

const GROUP = {
  __typename: "SandboxGroup",
  id: "sbg-created",
  name: "ci",
  region: "frankfurt",
  isDefault: true,
  environmentId: null,
  concurrencyLimit: 1000,
};

const onOpenChange = vi.fn();

beforeEach(() => {
  mutate.mockReset();
  copy.mockReset();
  onOpenChange.mockReset();
  index.retry.mockReset();
  owners.length = 0;
  placement.restrictsProtected = false;
  readEnvironments();
});

/** The dialog open, like the overview's `?new=sandbox-group`. */
function renderOpen() {
  return render(<NewSandboxGroupDialog open onOpenChange={onOpenChange} />);
}

/** Owns `open` the way the overview's URL does, for reopen behaviour. */
function Harness({ startOpen = true }: { startOpen?: boolean }) {
  const [open, setOpen] = useState(startOpen);
  return (
    <>
      <button onClick={() => setOpen(true)}>reopen</button>
      <NewSandboxGroupDialog open={open} onOpenChange={setOpen} />
    </>
  );
}

describe("NewSandboxGroupDialog", () => {
  it("asks for a name only when the workspace has no environments: no region field (bex-api places the group) and no environment select", () => {
    renderOpen();
    const dialog = screen.getByRole("dialog", { name: "New Sandbox Group" });

    expect(within(dialog).getByLabelText("Name")).toHaveValue("");
    expect(within(dialog).queryByLabelText("Region")).not.toBeInTheDocument();
    expect(within(dialog).queryByText(/oregon/i)).not.toBeInTheDocument();
    expect(within(dialog).queryByText(/environment/i)).not.toBeInTheDocument();
    expect(
      within(dialog).queryByRole("combobox", { name: "Environment" }),
    ).not.toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    ).toBeDisabled();
  });

  it("creates the group and confirms it in place: id, name, region, default, concurrency limit", async () => {
    mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
    const user = userEvent.setup();
    renderOpen();
    const dialog = screen.getByRole("dialog");

    await user.type(within(dialog).getByLabelText("Name"), "  ci ");
    await user.click(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    );

    // bex-api refuses a padded name, so the dialog sends it trimmed.
    expect(mutate).toHaveBeenCalledWith({
      variables: { name: "ci", ownerId: "tea-owner" },
    });
    expect(
      await within(dialog).findByText("Sandbox group created"),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("sbg-created")).toBeInTheDocument();
    expect(within(dialog).getByText("ci", { selector: "span" })).toBeVisible();
    // The region bex-api reports, as service settings show theirs.
    expect(within(dialog).getByText("frankfurt")).toBeVisible();
    expect(within(dialog).getByText("Yes")).toBeVisible();
    expect(within(dialog).getByText("1000")).toBeVisible();
    // The form is gone, and nothing on the confirmation links anywhere.
    expect(within(dialog).queryByLabelText("Name")).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("link")).not.toBeInTheDocument();

    await user.click(
      within(dialog).getByRole("button", { name: "Copy sandbox group ID" }),
    );
    expect(copy).toHaveBeenCalledWith("sbg-created");

    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("marks a group that is not the workspace default", async () => {
    mutate.mockResolvedValue({
      data: { createSandboxGroup: { ...GROUP, isDefault: false } },
    });
    const user = userEvent.setup();
    renderOpen();
    const dialog = screen.getByRole("dialog");

    await user.type(within(dialog).getByLabelText("Name"), "ci");
    await user.click(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    );

    expect(await within(dialog).findByText("No")).toBeVisible();
  });

  it("shows a duplicate name inline under the field and keeps the form", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("SANDBOX_GROUP_NAME_TAKEN", { field: "name" }),
    );
    const user = userEvent.setup();
    renderOpen();
    const dialog = screen.getByRole("dialog");
    const name = within(dialog).getByLabelText("Name");

    await user.type(name, "ci");
    await user.click(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    );

    const error = await within(dialog).findByRole("alert");
    expect(error).toHaveTextContent(
      'A sandbox group named "ci" already exists in this workspace.',
    );
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(name).toHaveAccessibleDescription(
      expect.stringContaining("already exists"),
    );
    expect(name).toHaveValue("ci");
    expect(within(dialog).queryByText("Sandbox group created")).toBeNull();

    // Editing the name withdraws the stale refusal.
    await user.type(name, "2");
    expect(within(dialog).queryByRole("alert")).not.toBeInTheDocument();
    expect(name).toHaveAttribute("aria-invalid", "false");
  });

  it("shows the 10-group limit as an inline alert and keeps the form", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("sandbox_group_limit_reached", { limit: 10 }),
    );
    const user = userEvent.setup();
    renderOpen();
    const dialog = screen.getByRole("dialog");

    await user.type(within(dialog).getByLabelText("Name"), "eleventh");
    await user.click(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    );

    const alert = await within(dialog).findByRole("alert");
    expect(alert).toHaveTextContent("Sandbox group limit reached");
    expect(alert).toHaveTextContent(
      "This workspace already has the maximum of 10 sandbox groups.",
    );
    expect(within(dialog).getByLabelText("Name")).toHaveValue("eleventh");
    expect(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    ).toBeEnabled();
  });

  it("submits on Enter", async () => {
    mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
    const user = userEvent.setup();
    renderOpen();

    await user.type(screen.getByLabelText("Name"), "ci{Enter}");

    expect(mutate).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("Sandbox group created")).toBeVisible();
  });

  it("does not submit a blank or whitespace-only name", async () => {
    const user = userEvent.setup();
    renderOpen();

    await user.type(screen.getByLabelText("Name"), "   {Enter}");

    expect(mutate).not.toHaveBeenCalled();
  });

  it("cancels through onOpenChange", async () => {
    const user = userEvent.setup();
    renderOpen();

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("holds the dialog open while a create is in flight", async () => {
    let finish: (value: unknown) => void = () => {};
    mutate.mockReturnValue(new Promise((resolve) => (finish = resolve)));
    const user = userEvent.setup();
    renderOpen();

    await user.type(screen.getByLabelText("Name"), "ci");
    await user.click(
      screen.getByRole("button", { name: "Create Sandbox Group" }),
    );
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    await user.keyboard("{Escape}");
    expect(onOpenChange).not.toHaveBeenCalled();

    finish({ data: { createSandboxGroup: GROUP } });
    expect(await screen.findByText("Sandbox group created")).toBeVisible();
  });

  it("reopens on a clean form, never on the last confirmation", async () => {
    mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
    const user = userEvent.setup();
    render(<Harness />);

    await user.type(screen.getByLabelText("Name"), "ci");
    await user.click(
      screen.getByRole("button", { name: "Create Sandbox Group" }),
    );
    await user.click(await screen.findByRole("button", { name: "Done" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "reopen" }));
    expect(await screen.findByLabelText("Name")).toHaveValue("");
    expect(screen.queryByText("Sandbox group created")).not.toBeInTheDocument();
  });

  describe("optional project environment (w22/m412)", () => {
    const SHOP = [
      environment("evm-prod", "Production"),
      environment("evm-stage", "Staging"),
    ];

    it("reads the workspace's environments only while it is open", async () => {
      const user = userEvent.setup();
      render(<Harness startOpen={false} />);
      expect(owners.at(-1)).toBeNull();

      await user.click(screen.getByRole("button", { name: "reopen" }));
      expect(owners.at(-1)).toBe("tea-owner");
    });

    it("offers an Environment select, on No environment, when the workspace has environments", () => {
      readEnvironments(...SHOP);
      renderOpen();

      const select = screen.getByRole("combobox", { name: "Environment" });
      expect(select).toHaveTextContent("No environment");
      expect(select).toBeEnabled();
      // It spans the form like the Name input above it.
      expect(select).toHaveClass("w-full");
    });

    it("sends no environmentId while the select is left on No environment", async () => {
      readEnvironments(...SHOP);
      mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
      const user = userEvent.setup();
      renderOpen();

      await user.type(screen.getByLabelText("Name"), "ci");
      await user.click(
        screen.getByRole("button", { name: "Create Sandbox Group" }),
      );

      expect(mutate.mock.calls[0]?.[0]).toStrictEqual({
        variables: { name: "ci", ownerId: "tea-owner" },
      });
      // The confirmation says so rather than leaving the row out.
      const dialog = screen.getByRole("dialog");
      expect(await within(dialog).findByText("Environment")).toBeVisible();
      expect(within(dialog).getByText("No environment")).toBeVisible();
    });

    it("places the group in the chosen environment and confirms it by its project and name", async () => {
      readEnvironments(...SHOP);
      mutate.mockResolvedValue({
        data: { createSandboxGroup: { ...GROUP, environmentId: "evm-stage" } },
      });
      const user = userEvent.setup();
      renderOpen();
      const dialog = screen.getByRole("dialog");

      await user.type(within(dialog).getByLabelText("Name"), "ci");
      await user.click(
        within(dialog).getByRole("combobox", { name: "Environment" }),
      );
      await user.click(screen.getByRole("option", { name: "Shop / Staging" }));
      await user.click(
        within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
      );

      expect(mutate).toHaveBeenCalledWith({
        variables: {
          name: "ci",
          ownerId: "tea-owner",
          environmentId: "evm-stage",
        },
      });
      expect(await within(dialog).findByText("Shop / Staging")).toBeVisible();
      expect(within(dialog).queryByText("evm-stage")).not.toBeInTheDocument();
    });

    it("falls back to the id the server reports when the index lacks the environment", async () => {
      readEnvironments(...SHOP);
      mutate.mockResolvedValue({
        data: { createSandboxGroup: { ...GROUP, environmentId: "evm-other" } },
      });
      const user = userEvent.setup();
      renderOpen();

      await user.type(screen.getByLabelText("Name"), "ci{Enter}");

      expect(await screen.findByText("evm-other")).toBeVisible();
    });

    it("shows a loading state where the select will be and holds Create until it is read", async () => {
      index.ready = false;
      index.loading = true;
      const user = userEvent.setup();
      const view = renderOpen();

      const loading = screen.getByRole("status", {
        name: "Loading environments",
      });
      expect(loading).toBeVisible();
      expect(
        screen.queryByRole("combobox", { name: "Environment" }),
      ).not.toBeInTheDocument();
      await user.type(screen.getByLabelText("Name"), "ci{Enter}");
      expect(
        screen.getByRole("button", { name: "Create Sandbox Group" }),
      ).toBeDisabled();
      expect(mutate).not.toHaveBeenCalled();

      readEnvironments(...SHOP);
      view.rerender(<NewSandboxGroupDialog open onOpenChange={onOpenChange} />);
      expect(
        screen.queryByRole("status", { name: "Loading environments" }),
      ).not.toBeInTheDocument();
      expect(
        screen.getByRole("combobox", { name: "Environment" }),
      ).toBeVisible();
      expect(
        screen.getByRole("button", { name: "Create Sandbox Group" }),
      ).toBeEnabled();
    });

    it("lets a group be created without an environment when the environments could not be read", async () => {
      index.ready = false;
      index.loading = false;
      index.error = new Error("environment index failed");
      mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
      const user = userEvent.setup();
      renderOpen();

      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("Couldn't load your environments.");
      await user.click(
        within(alert).getByRole("button", { name: "Try again" }),
      );
      expect(index.retry).toHaveBeenCalledTimes(1);

      await user.type(screen.getByLabelText("Name"), "ci");
      await user.click(
        screen.getByRole("button", { name: "Create Sandbox Group" }),
      );
      expect(mutate.mock.calls[0]?.[0]).toStrictEqual({
        variables: { name: "ci", ownerId: "tea-owner" },
      });
    });

    it("disables a protected environment for a non-Admin and says why", async () => {
      readEnvironments(
        environment("evm-prod", "Production", "protected"),
        environment("evm-stage", "Staging"),
      );
      placement.restrictsProtected = true;
      const user = userEvent.setup();
      renderOpen();

      await user.click(screen.getByRole("combobox", { name: "Environment" }));
      expect(
        screen.getByRole("option", {
          name: "Shop / Production (admins only)",
        }),
      ).toHaveAttribute("aria-disabled", "true");
      expect(
        screen.getByRole("option", { name: "Shop / Staging" }),
      ).not.toHaveAttribute("aria-disabled", "true");
      await user.keyboard("{Escape}");
      expect(
        screen.getByText(
          "Only an admin can create resources in a protected environment.",
        ),
      ).toBeVisible();
    });

    it("offers a protected environment to an Admin", async () => {
      readEnvironments(environment("evm-prod", "Production", "protected"));
      const user = userEvent.setup();
      renderOpen();

      await user.click(screen.getByRole("combobox", { name: "Environment" }));
      expect(
        screen.getByRole("option", { name: "Shop / Production" }),
      ).not.toHaveAttribute("aria-disabled", "true");
    });

    it("shows a raced protected-placement refusal under the select and keeps the form", async () => {
      readEnvironments(environment("evm-prod", "Production", "protected"));
      mutate.mockRejectedValue(
        codedGraphQLError("PROTECTED_ENVIRONMENT_ADMIN_REQUIRED"),
      );
      const user = userEvent.setup();
      renderOpen();
      const dialog = screen.getByRole("dialog");

      await user.type(within(dialog).getByLabelText("Name"), "ci");
      await user.click(
        within(dialog).getByRole("combobox", { name: "Environment" }),
      );
      await user.click(
        screen.getByRole("option", { name: "Shop / Production" }),
      );
      await user.click(
        within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
      );

      expect(await within(dialog).findByRole("alert")).toHaveTextContent(
        "Only an admin can create resources in a protected environment or move them into or out of it.",
      );
      expect(within(dialog).getByLabelText("Name")).toHaveValue("ci");
      expect(
        within(dialog).getByRole("combobox", { name: "Environment" }),
      ).toHaveTextContent("Shop / Production");
    });

    it("re-reads the environments when the chosen one is gone, and falls back to No environment", async () => {
      readEnvironments(...SHOP);
      mutate.mockRejectedValue(codedGraphQLError("NOT_FOUND"));
      const user = userEvent.setup();
      const view = renderOpen();
      const dialog = screen.getByRole("dialog");

      await user.type(within(dialog).getByLabelText("Name"), "ci");
      await user.click(
        within(dialog).getByRole("combobox", { name: "Environment" }),
      );
      await user.click(screen.getByRole("option", { name: "Shop / Staging" }));
      await user.click(
        within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
      );

      expect(await within(dialog).findByRole("alert")).toHaveTextContent(
        "That environment no longer exists. Choose another or none.",
      );
      expect(index.retry).toHaveBeenCalledTimes(1);

      // The re-read no longer lists it: the select shows No environment and
      // would send none, never the id that is gone.
      readEnvironments(environment("evm-prod", "Production"));
      view.rerender(<NewSandboxGroupDialog open onOpenChange={onOpenChange} />);
      expect(
        within(dialog).getByRole("combobox", { name: "Environment" }),
      ).toHaveTextContent("No environment");
      mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
      await user.click(
        within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
      );
      expect(mutate.mock.calls.at(-1)?.[0]).toStrictEqual({
        variables: { name: "ci", ownerId: "tea-owner" },
      });
    });

    it("locks the select while a create is in flight", async () => {
      readEnvironments(...SHOP);
      mutate.mockReturnValue(new Promise(() => {}));
      const user = userEvent.setup();
      renderOpen();

      await user.type(screen.getByLabelText("Name"), "ci");
      await user.click(
        screen.getByRole("button", { name: "Create Sandbox Group" }),
      );

      expect(
        screen.getByRole("combobox", { name: "Environment" }),
      ).toBeDisabled();
    });

    it("reopens on No environment, never on the last choice", async () => {
      readEnvironments(...SHOP);
      mutate.mockResolvedValue({
        data: { createSandboxGroup: { ...GROUP, environmentId: "evm-stage" } },
      });
      const user = userEvent.setup();
      render(<Harness />);

      await user.type(screen.getByLabelText("Name"), "ci");
      await user.click(screen.getByRole("combobox", { name: "Environment" }));
      await user.click(screen.getByRole("option", { name: "Shop / Staging" }));
      await user.click(
        screen.getByRole("button", { name: "Create Sandbox Group" }),
      );
      await user.click(await screen.findByRole("button", { name: "Done" }));

      await user.click(screen.getByRole("button", { name: "reopen" }));
      expect(
        await screen.findByRole("combobox", { name: "Environment" }),
      ).toHaveTextContent("No environment");
    });
  });
});
