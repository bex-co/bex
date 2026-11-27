import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import {
  ResumeServiceDocument,
  SuspendDatabaseDocument,
  SuspendServiceDocument,
  ViewerCapabilitiesDocument,
} from "@/graphql/definitions";
import { BulkResourceToolbar } from "@/features/projects/components/bulk-resource-toolbar";
import { useResourceSelection } from "@/features/projects/hooks/use-resource-selection";
import { MoveDisabledContext } from "@/features/projects/lib/move-disabled";
import {
  resourceSelectionKey,
  type ResourceRow,
} from "@/features/projects/types";
import type { ServiceView } from "@/features/services/types";
import type { DatabaseView } from "@/features/databases/types";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { Capabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { codedGraphQLError } from "@/test/mocks/apollo";

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
});

const client = vi.hoisted(() => ({
  query: vi.fn(),
  mutate: vi.fn(),
  refetchQueries: vi.fn(),
}));
vi.mock("@apollo/client/react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@apollo/client/react")>()),
  useApolloClient: () => client,
}));
const capabilities = vi.hoisted(() => ({ current: null as unknown }));
vi.mock(
  "@/features/capabilities/hooks/use-capabilities",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("@/features/capabilities/hooks/use-capabilities")
    >()),
    useCapabilities: () => capabilities.current,
  }),
);
vi.mock("@/features/workspaces/context/hooks", async () => {
  const { mockWorkspaceContext } = await import("@/test/mocks/workspace");
  return mockWorkspaceContext("tea-test");
});
// Move is the environment card's tested path; these tests are about
// Suspend and Resume, so Move's writers stay inert.
vi.mock("@/features/projects/hooks/use-bulk-move", () => ({
  useBulkMove: () => ({
    move: vi.fn(),
    movingTo: () => false,
    strandedPrompt: null,
    confirmStranded: vi.fn(),
    dismissStranded: vi.fn(),
  }),
}));

const ADMIN = mockCapabilities();
const DEVELOPER = mockCapabilities({ role: "DEVELOPER", canManage: false });
const VIEWER = mockCapabilities({
  role: "VIEWER",
  canOperate: false,
  canCreate: false,
  canManage: false,
});

function grantsFor(caps: Capabilities) {
  return {
    data: {
      viewerCapabilities: {
        role: caps.role,
        grants: (["can_operate", "can_manage"] as const).map((action) => ({
          action,
          outcome: caps.allows(action) ? "allowed" : "denied",
          reason: null,
        })),
      },
    },
  };
}

function service(
  id: string,
  type: ServiceView["type"],
  suspended: boolean,
): ResourceRow {
  return {
    kind: "service",
    id,
    name: id,
    createdAt: null,
    updatedAt: null,
    runtime: null,
    region: null,
    service: { id, name: id, type, suspended } as ServiceView,
  };
}

const web = service("web", "web_service", false);
const worker = service("worker", "background_worker", true);
const site = service("site", "static_site", false);
const postgres: ResourceRow = {
  kind: "database",
  id: "pg",
  name: "pg",
  createdAt: null,
  updatedAt: null,
  runtime: null,
  region: null,
  database: {
    id: "pg",
    name: "pg",
    suspended: "not_suspended",
  } as DatabaseView,
};

function Harness({
  rows,
  protectedEnvironment = false,
}: {
  rows: ResourceRow[];
  protectedEnvironment?: boolean;
}) {
  const selection = useResourceSelection(rows);
  return (
    <>
      <BulkResourceToolbar
        selection={selection}
        moveTargets={[]}
        protectedEnvironment={protectedEnvironment}
      />
      <button
        onClick={() =>
          selection.setSelected(new Set(rows.map(resourceSelectionKey)))
        }
      >
        select all
      </button>
      <ul aria-label="selected">
        {selection.selectedRows.map((row) => (
          <li key={row.id}>{row.name}</li>
        ))}
      </ul>
    </>
  );
}

const staging = {
  id: "evm-staging",
  projectId: "prj-test",
  name: "staging",
  protectedStatus: "unprotected",
} as EnvironmentView;

/** The Overview's shape: a selection that may move into another environment. */
function MoveHarness({ rows }: { rows: ResourceRow[] }) {
  const selection = useResourceSelection(rows);
  return (
    <>
      <BulkResourceToolbar
        selection={selection}
        moveTargets={[{ environment: staging, label: "project / staging" }]}
      />
      <button
        onClick={() =>
          selection.setSelected(new Set(rows.map(resourceSelectionKey)))
        }
      >
        select all
      </button>
    </>
  );
}

async function selectAll(rows: ResourceRow[], protectedEnvironment = false) {
  const user = userEvent.setup();
  render(<Harness rows={rows} protectedEnvironment={protectedEnvironment} />);
  await user.click(screen.getByRole("button", { name: "select all" }));
  return user;
}

function selectedNames() {
  return within(screen.getByRole("list", { name: "selected" }))
    .queryAllByRole("listitem")
    .map((item) => item.textContent);
}

beforeEach(() => {
  capabilities.current = ADMIN;
  client.query.mockReset();
  client.query.mockResolvedValue(grantsFor(ADMIN));
  client.mutate.mockReset();
  client.mutate.mockResolvedValue({ data: {} });
  client.refetchQueries.mockReset();
  client.refetchQueries.mockResolvedValue([]);
});

describe("BulkResourceToolbar Suspend and Resume (w4/m550)", () => {
  it("offers nothing until a resource is selected", () => {
    render(<Harness rows={[web, worker]} />);
    expect(screen.queryByRole("button", { name: "Suspend" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Resume" })).toBeNull();
  });

  it("suspends the running half of a mixed selection, one call per service", async () => {
    const user = await selectAll([web, worker, site]);
    // A mixed selection shows both actions, as Render's toolbar does.
    expect(screen.getByRole("button", { name: "Resume" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "Suspend" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByRole("heading", { name: "Suspend services" }),
    ).toBeInTheDocument();
    // The worker is already suspended: Render leaves it out of the list.
    expect(within(dialog).getByText("web")).toBeInTheDocument();
    expect(within(dialog).getByText("site")).toBeInTheDocument();
    expect(within(dialog).queryByText("worker")).toBeNull();

    const confirm = within(dialog).getByRole("button", {
      name: "Suspend services",
    });
    expect(confirm).toBeDisabled();
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo suspend 2 services",
    );
    await user.click(confirm);

    await waitFor(() =>
      expect(
        within(dialog).getByRole("button", { name: "Close" }),
      ).toBeInTheDocument(),
    );
    expect(client.mutate).toHaveBeenCalledTimes(2);
    for (const id of ["web", "site"]) {
      expect(client.mutate).toHaveBeenCalledWith({
        mutation: SuspendServiceDocument,
        variables: { id },
      });
    }
    // The grant is read fresh before anything is sent.
    expect(client.query).toHaveBeenCalledWith(
      expect.objectContaining({
        query: ViewerCapabilitiesDocument,
        variables: { ownerId: "tea-test", fresh: true },
      }),
    );
    expect(client.refetchQueries).toHaveBeenCalledWith({
      include: ["Services", "Databases", "KeyValues"],
    });

    // Every row changed, so Close clears them from the selection; the worker
    // the dialog never touched stays selected.
    await user.click(within(dialog).getByRole("button", { name: "Close" }));
    expect(selectedNames()).toEqual(["worker"]);
  });

  it("resumes only the suspended rows, with no typed phrase", async () => {
    const user = await selectAll([web, worker]);
    await user.click(screen.getByRole("button", { name: "Resume" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).queryByRole("textbox")).toBeNull();
    expect(within(dialog).queryByText("web")).toBeNull();
    await user.click(
      within(dialog).getByRole("button", { name: "Resume services" }),
    );
    await waitFor(() => expect(client.mutate).toHaveBeenCalledTimes(1));
    expect(client.mutate).toHaveBeenCalledWith({
      mutation: ResumeServiceDocument,
      variables: { id: "worker" },
    });
  });

  it("keeps a failed row selected with its reason, while the others complete", async () => {
    client.mutate.mockImplementation(
      async ({ variables }: { variables: { id: string } }) => {
        if (variables.id === "pg")
          throw new CombinedGraphQLErrors({
            errors: [
              { message: "conflict: database pg is still being created" },
            ],
          } as never);
        return { data: {} };
      },
    );
    const user = await selectAll([web, postgres]);
    await user.click(screen.getByRole("button", { name: "Suspend" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo suspend 1 service and 1 database",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Suspend services" }),
    );

    const retry = await within(dialog).findByRole("button", {
      name: "Retry pg",
    });
    expect(within(dialog).getByText("Suspended")).toBeInTheDocument();
    expect(client.mutate).toHaveBeenCalledWith({
      mutation: SuspendDatabaseDocument,
      variables: { id: "pg" },
    });
    expect(
      within(dialog).getByText("Database pg is still being created"),
    ).toBeInTheDocument();

    // Retry runs that one row again; it still fails.
    client.mutate.mockClear();
    await user.click(retry);
    await waitFor(() => expect(client.mutate).toHaveBeenCalledTimes(1));
    expect(client.mutate).toHaveBeenCalledWith({
      mutation: SuspendDatabaseDocument,
      variables: { id: "pg" },
    });

    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(selectedNames()).toEqual(["pg"]);
  });

  it("disables both actions for a Developer in a protected environment, with Render's reason", async () => {
    capabilities.current = DEVELOPER;
    const user = await selectAll([web, worker], true);
    const suspend = screen.getByRole("button", { name: "Suspend" });
    const resume = screen.getByRole("button", { name: "Resume" });
    expect(suspend).toBeDisabled();
    expect(resume).toBeDisabled();
    await user.hover(suspend.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "This action is limited to admins.",
    );
    await user.click(suspend);
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(client.mutate).not.toHaveBeenCalled();
  });

  it("lets a Developer act outside a protected environment", async () => {
    capabilities.current = DEVELOPER;
    await selectAll([web], false);
    expect(screen.getByRole("button", { name: "Suspend" })).toBeEnabled();
  });

  it("gives a Viewer no enabled lifecycle action", async () => {
    capabilities.current = VIEWER;
    await selectAll([web, worker]);
    expect(screen.getByRole("button", { name: "Suspend" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Resume" })).toBeDisabled();
  });

  // bex-api refuses a Viewer's placement write (can_create), and Render
  // disables Move without its moveResource permission: the picker and Move
  // stay disabled with the role's reason, rather than answering "Forbidden"
  // after the click (the m550 dev-4 replay, on the Overview's ungrouped list).
  it.each([
    ["a Viewer", VIEWER, true],
    ["a Developer", DEVELOPER, false],
  ])("gates Move on the role for %s", async (_label, caps, blocked) => {
    capabilities.current = caps;
    const user = userEvent.setup();
    render(<MoveHarness rows={[web]} />);
    await user.click(screen.getByRole("button", { name: "select all" }));
    const picker = screen.getByRole("combobox", {
      name: "Move selected resources to environment",
    });
    const move = screen.getByRole("button", { name: "Move" });
    if (blocked) {
      expect(picker).toBeDisabled();
      expect(move).toBeDisabled();
      await user.hover(move.parentElement!);
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        "Your role can’t make this change. Ask a workspace admin.",
      );
    } else {
      expect(picker).toBeEnabled();
    }
  });

  // A Blueprint's Resources page (w3/m680): Render's action bar there has
  // Resume and Suspend and no Move (disableMove).
  it("offers Resume and Suspend and no Move in a list whose rows are not moved", async () => {
    const user = userEvent.setup();
    render(
      <MoveDisabledContext.Provider value>
        <MoveHarness rows={[web, worker]} />
      </MoveDisabledContext.Provider>,
    );
    await user.click(screen.getByRole("button", { name: "select all" }));
    expect(screen.getByText("Selected: 2")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Suspend" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Resume" })).toBeEnabled();
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.queryByRole("button", { name: "Move" })).toBeNull();
  });

  it("asks an Admin in a protected environment once, for the whole selection", async () => {
    const user = await selectAll([web, site], true);
    await user.click(screen.getByRole("button", { name: "Suspend" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo suspend 2 services",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Suspend services" }),
    );
    await waitFor(() =>
      expect(
        within(dialog).getByRole("button", { name: "Close" }),
      ).toBeInTheDocument(),
    );
    expect(client.mutate).toHaveBeenCalledTimes(2);
    expect(screen.getAllByRole("alertdialog")).toHaveLength(1);
  });

  // w4/728: a row the server refuses because the caller is no longer an Admin
  // of the protected environment reads Render's reason, not the server's.
  it("reads a row's raced admin-only refusal as Render's reason", async () => {
    client.mutate.mockImplementation(
      async ({ variables }: { variables: { id: string } }) => {
        if (variables.id === "site")
          throw codedGraphQLError(
            "PROTECTED_ENVIRONMENT_ADMIN_REQUIRED",
            { environmentId: "evm-1" },
            "only workspace Admins can suspend resources in a protected environment",
          );
        return { data: {} };
      },
    );
    const user = await selectAll([web, site], true);
    await user.click(screen.getByRole("button", { name: "Suspend" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo suspend 2 services",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Suspend services" }),
    );
    expect(
      await within(dialog).findByText("This action is limited to admins."),
    ).toBeInTheDocument();
    expect(within(dialog).queryByText(/only workspace Admins/)).toBeNull();
  });

  it("sends nothing when the role changed between selection and confirmation", async () => {
    // The toolbar read an Admin; the fresh read at dispatch is a Developer's.
    client.query.mockResolvedValue(grantsFor(DEVELOPER));
    const user = await selectAll([web], true);
    await user.click(screen.getByRole("button", { name: "Suspend" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo suspend 1 service",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Suspend services" }),
    );
    await waitFor(() => expect(client.query).toHaveBeenCalled());
    expect(client.mutate).not.toHaveBeenCalled();
    expect(
      within(dialog).getByRole("button", { name: "Suspend services" }),
    ).toBeInTheDocument();
  });
});
