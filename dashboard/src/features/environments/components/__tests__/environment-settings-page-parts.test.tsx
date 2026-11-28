import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { EnvGroupView } from "@/features/env-groups/types";
import { environmentSettingsSections } from "@/features/environments/lib/settings";
import { EnvironmentSettingsNavigation } from "@/features/environments/components/environment-settings-navigation";
import { EnvironmentSettingsCardsSkeleton } from "@/features/environments/components/environment-settings-skeleton";
import { EnvironmentNetworkingCard } from "@/features/environments/components/environment-networking-card";
import { EnvironmentEnvGroupsCard } from "@/features/environments/components/environment-env-groups-card";

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@tanstack/react-router")>();
  const { PlainLink } = await import("@/test/mocks/router-link");
  return { ...actual, Link: PlainLink, useNavigate: () => vi.fn() };
});

const update = vi.fn();
vi.mock("@/features/environments/hooks/use-update-environment", () => ({
  useUpdateEnvironment: () => ({ update, saving: false }),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const groups = vi.hoisted(() => ({
  list: [] as EnvGroupView[],
  loading: false,
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({
    groups: groups.list,
    loading: groups.loading,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
const createDialog = vi.hoisted(() => vi.fn());
vi.mock("@/features/env-groups/components/new-env-group-dialog", () => ({
  NewEnvGroupDialog: (props: { initialEnvironmentId?: string }) => {
    createDialog(props.initialEnvironmentId);
    return <div role="dialog">create env group</div>;
  },
}));
vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => ({
  useEnvGroupScopeIndex: () => ({
    environments: [],
    projects: [],
    serviceEnvironmentById: new Map(),
    loading: false,
    ready: true,
    error: undefined,
    retry: vi.fn(),
  }),
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
  }),
}));
vi.mock("@/features/projects/components/resource-table", () => ({
  ResourceTable: ({ rows }: { rows: Array<{ name: string }> }) => (
    <table>
      <tbody>
        {rows.map((row) => (
          <tr key={row.name}>
            <td>{row.name}</td>
          </tr>
        ))}
      </tbody>
    </table>
  ),
}));

const allowAll = [
  { cidrBlock: "0.0.0.0/0", description: "Allow all (default)" },
  { cidrBlock: "::/0", description: "Allow all (default)" },
];

const staging: EnvironmentView = {
  id: "evm-staging",
  projectId: "prj-shop",
  name: "staging",
  ownerId: "tea-1",
  createdAt: null,
  serviceIds: [],
  databaseIds: [],
  keyValueIds: [],
  envGroupIds: ["evg-a", "evg-b"],
  protectedStatus: "unprotected",
  networkIsolationEnabled: false,
  ipAllowListEntries: allowAll,
};

const group = (id: string, name: string): EnvGroupView => ({
  id,
  name,
  ownerId: "tea-1",
  environmentId: staging.id,
  createdAt: null,
  updatedAt: null,
  revision: "1",
  availability: null,
  serviceLinks: [],
  envVarKeys: [],
  secretFileNames: [],
});

beforeEach(() => {
  update.mockReset();
  update.mockResolvedValue({ ok: true });
  groups.list = [];
  groups.loading = false;
  createDialog.mockReset();
});

describe("environment settings sections (w1/m683)", () => {
  it("shows Permissions, Connections and Networking only to an Admin, as Render shows them to a member who may change them", () => {
    expect(environmentSettingsSections(false)).toEqual([
      "name",
      "permissions",
      "cross-environment-connections",
      "environment-groups",
      "networking",
      "delete-environment",
    ]);
    expect(environmentSettingsSections(true)).toEqual([
      "name",
      "environment-groups",
      "delete-environment",
    ]);
  });

  it("navigates to each section by Render's anchors, in page order", () => {
    render(
      <EnvironmentSettingsNavigation
        sections={environmentSettingsSections(false)}
      />,
    );
    const nav = screen.getByRole("navigation", {
      name: "Environment settings sections",
    });
    expect(
      within(nav)
        .getAllByRole("link")
        .map((link) => [link.textContent, link.getAttribute("href")]),
    ).toEqual([
      ["Name", "#name"],
      ["Permissions", "#permissions"],
      ["Connections", "#cross-environment-connections"],
      ["Environment Groups", "#environment-groups"],
      ["Networking", "#networking"],
      ["Delete Environment", "#delete-environment"],
    ]);
  });

  it.each([false, true])(
    "previews exactly the sections the page shows (non-Admin: %s)",
    (adminDenied) => {
      const sections = environmentSettingsSections(adminDenied);
      const { container } = render(
        <EnvironmentSettingsCardsSkeleton
          sections={sections}
          connectionsGated={false}
          belowScale={false}
        />,
      );
      const regions = [
        ...container.querySelectorAll(":scope > [data-skeleton-region]"),
      ].map((region) => region.getAttribute("data-skeleton-region"));
      expect(regions).toEqual(sections);
    },
  );

  it("previews the plan gates the page is about to show", () => {
    const { container, rerender } = render(
      <EnvironmentSettingsCardsSkeleton
        sections={environmentSettingsSections(false)}
        connectionsGated
        belowScale
      />,
    );
    const text = () => container.textContent ?? "";
    expect(text()).toContain(
      "Block cross-environment connections by upgrading to the Pro plan.",
    );
    expect(text()).toContain("Open to all source IPs");
    rerender(
      <EnvironmentSettingsCardsSkeleton
        sections={environmentSettingsSections(false)}
        connectionsGated={false}
        belowScale={false}
      />,
    );
    expect(text()).not.toContain("by upgrading to the Pro plan");
    expect(text()).not.toContain("Open to all source IPs");
  });
});

describe("EnvironmentNetworkingCard (w1/m683)", () => {
  it("edits the environment's inbound IP rules on Scale, saving only that field", async () => {
    const user = userEvent.setup();
    render(
      <EnvironmentNetworkingCard environment={staging} belowScale={false} />,
    );
    await user.click(screen.getByRole("button", { name: "Remove ::/0" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(update).toHaveBeenCalledWith("evm-staging", {
      ipAllowListEntries: [allowAll[0]],
    });
  });

  it("below Scale lists the rules and offers only the reset to allow all", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <EnvironmentNetworkingCard environment={staging} belowScale />,
    );
    expect(screen.getByText("Open to all source IPs")).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.getByRole("link", { name: "Upgrade" })).toHaveAttribute(
      "href",
      "/billing/update-plan",
    );

    rerender(
      <EnvironmentNetworkingCard
        environment={{ ...staging, ipAllowListEntries: [] }}
        belowScale
      />,
    );
    await user.click(
      screen.getByRole("button", { name: "Reset to allow all" }),
    );
    expect(update).toHaveBeenCalledWith("evm-staging", {
      ipAllowListEntries: allowAll,
    });
  });
});

describe("EnvironmentEnvGroupsCard (w1/m683)", () => {
  it("lists the environment's own env groups and searches them", async () => {
    const user = userEvent.setup();
    groups.list = [
      group("evg-a", "shared"),
      group("evg-b", "secrets"),
      group("evg-other", "elsewhere"),
    ];
    render(<EnvironmentEnvGroupsCard environment={staging} />);
    expect(screen.getByText("shared")).toBeInTheDocument();
    expect(screen.getByText("secrets")).toBeInTheDocument();
    expect(screen.queryByText("elsewhere")).toBeNull();

    await user.type(
      screen.getByRole("searchbox", { name: "Search Environment Groups" }),
      "sec",
    );
    expect(screen.queryByText("shared")).toBeNull();
    expect(screen.getByText("secrets")).toBeInTheDocument();
  });

  it("says Render's sentence when the environment has none", () => {
    render(
      <EnvironmentEnvGroupsCard
        environment={{ ...staging, envGroupIds: [] }}
      />,
    );
    expect(screen.getByText("No environment groups found")).toBeInTheDocument();
  });

  it("opens the create dialog with this environment chosen", async () => {
    const user = userEvent.setup();
    render(<EnvironmentEnvGroupsCard environment={staging} />);
    await user.click(screen.getByRole("button", { name: "New Env Group" }));
    expect(createDialog).toHaveBeenCalledWith("evm-staging");
  });

  it("disables New Env Group with the reason for a member who may not create here", () => {
    render(
      <EnvironmentEnvGroupsCard
        environment={staging}
        createReason="Only an admin can create resources in a protected environment."
      />,
    );
    expect(
      screen.getByRole("button", { name: "New Env Group" }),
    ).toBeDisabled();
  });
});
