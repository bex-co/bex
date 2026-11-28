import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { InboundIPLevels } from "@/common/components/inbound-ip-levels";
import {
  useInboundIPLevels,
  type InboundIPLevelsRead,
} from "@/common/hooks/use-inbound-ip-levels";
import {
  ALLOW_ALL_PAIR_ENTRIES,
  type IPAllowListEntryDraft,
} from "@/common/lib/ip-allow-list";
import { AccessControlPanel } from "@/features/databases/components/access-control-panel";
import { EnvironmentNetworkingCard } from "@/features/environments/components/environment-networking-card";
import { KeyValueNetworkingPanel } from "@/features/keyvalue/components/key-value-networking-panel";
import { ServiceNetworkingPanel } from "@/features/services/components/service-networking-panel";

const state = vi.hoisted(() => ({
  entries: [] as { cidrBlock: string; description: string }[],
  environmentId: null as string | null,
}));

vi.mock("@/features/workspaces/context", () => ({
  useWorkspace: () => ({ currentWorkspace: { id: "tea-1", plan: "scale" } }),
}));
vi.mock("@/features/services/hooks/use-service-networking", () => ({
  useServiceNetworking: () => ({ saving: false, saveAllowList: vi.fn() }),
}));
vi.mock("@/features/databases/hooks/use-access-control", () => ({
  useAccessControl: () => ({
    allowList: state.entries,
    environmentId: state.environmentId,
    isPublic: true,
    savingAllowList: false,
    saveAllowList: vi.fn(),
    credentials: [],
    pooled: null,
  }),
}));
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({ instanceTypes: [] }),
}));
vi.mock(
  "@/features/databases/hooks/use-update-database-connection-pool",
  () => ({
    useUpdateDatabaseConnectionPool: () => ({
      updateConnectionPool: vi.fn(),
      busy: false,
    }),
  }),
);
vi.mock("@/features/keyvalue/hooks/use-key-value-networking", () => ({
  useKeyValueNetworking: () => ({
    allowList: state.entries,
    environmentId: state.environmentId,
    isPublic: true,
    savingAllowList: false,
    saveAllowList: vi.fn(),
  }),
}));
vi.mock("@/features/environments/hooks/use-update-environment", () => ({
  useUpdateEnvironment: () => ({ update: vi.fn(), saving: false }),
}));

const rules = (...cidrs: string[]): IPAllowListEntryDraft[] =>
  cidrs.map((cidrBlock) => ({ cidrBlock, description: `${cidrBlock} note` }));

// The milestone's worked example (w4/m710 t003).
const workspace = rules("10.0.0.0/8");
const environment = rules("10.1.0.0/16");
const service = rules("10.1.2.0/24", "192.168.0.0/16");
const ENVIRONMENT_HREF = "/project/prj-1/environment/evm-1/settings#networking";

function levels(read: InboundIPLevelsRead) {
  vi.mocked(useInboundIPLevels).mockReturnValue(read);
}

const bothLevels: InboundIPLevelsRead = {
  state: "ready",
  workspace,
  environment: { entries: environment, href: ENVIRONMENT_HREF },
};

function renderLevels(draft: IPAllowListEntryDraft[] = service) {
  return render(
    <InboundIPLevels
      environmentId="evm-1"
      saved={service}
      draft={draft}
      ownType="Web Service"
    />,
  );
}

const section = (level: "workspace" | "environment") =>
  screen.getByTestId(`inbound-ip-affected-by-${level}`);

describe("InboundIPLevels", () => {
  it("shows each higher level read-only with Render's breadcrumb and a View rules link to where it is edited", () => {
    levels(bothLevels);
    renderLevels();

    const ws = within(section("workspace"));
    expect(ws.getByRole("heading", { name: "Affected by:" })).toBeVisible();
    expect(ws.getByRole("link", { name: "Workspace" })).toHaveAttribute(
      "href",
      "/network-access",
    );
    const wsRules = ws.getByRole("link", { name: "View rules" });
    expect(wsRules).toHaveAttribute("href", "/network-access");
    expect(wsRules).toHaveAttribute("target", "_blank");
    expect(ws.getByRole("columnheader", { name: "Source" })).toBeVisible();
    expect(ws.getByRole("cell", { name: "10.0.0.0/8" })).toBeVisible();
    expect(ws.getByRole("cell", { name: "10.0.0.0/8 note" })).toBeVisible();

    const env = within(section("environment"));
    // The workspace crumb is the path, not a link; Environment is.
    expect(env.queryByRole("link", { name: "Workspace" })).toBeNull();
    expect(env.getByText("Workspace")).toBeVisible();
    expect(env.getByRole("link", { name: "Environment" })).toHaveAttribute(
      "href",
      ENVIRONMENT_HREF,
    );
    const envRules = env.getByRole("link", { name: "View rules" });
    expect(envRules).toHaveAttribute("href", ENVIRONMENT_HREF);
    expect(envRules).toHaveAttribute("target", "_blank");
    expect(env.getByRole("cell", { name: "10.1.0.0/16" })).toBeVisible();

    expect(
      screen.getByRole("heading", { name: "Web Service Inbound IP Rules" }),
    ).toBeVisible();
  });

  it("warns that a level with no rule blocks all internet traffic, in place of its table", () => {
    levels({
      state: "ready",
      workspace: [],
      environment: { entries: [], href: ENVIRONMENT_HREF },
    });
    renderLevels();
    expect(
      within(section("workspace")).getByText(
        "All internet traffic is blocked by Workspace inbound IP rules.",
      ),
    ).toBeVisible();
    expect(within(section("workspace")).queryByRole("table")).toBeNull();
    expect(
      within(section("environment")).getByText(
        "All internet traffic is blocked by Environment inbound IP rules.",
      ),
    ).toBeVisible();
  });

  it("shows only the workspace level outside an environment", () => {
    levels({ state: "ready", workspace });
    renderLevels();
    expect(section("workspace")).toBeVisible();
    expect(
      screen.queryByTestId("inbound-ip-affected-by-environment"),
    ).toBeNull();
  });

  it("counts the ranges every saved level allows together", () => {
    levels(bothLevels);
    renderLevels();
    const status = screen.getByRole("status");
    expect(status).toHaveTextContent(
      "1 IP range is allowed from outside of your private network.",
    );

    levels({ state: "ready", workspace: ALLOW_ALL_PAIR_ENTRIES });
    render(
      <InboundIPLevels
        saved={rules("10.0.0.0/8", "192.168.0.0/16")}
        draft={[]}
        ownType="PostgreSQL"
      />,
    );
    expect(screen.getAllByRole("status")[1]).toHaveTextContent(
      "2 IP ranges are allowed from outside of your private network.",
    );
  });

  it("checks an address against every level, naming the first that refuses it", async () => {
    levels(bothLevels);
    const user = userEvent.setup();
    renderLevels();
    const field = screen.getByRole("textbox", { name: "Validate IP address" });
    expect(field).toHaveAttribute("placeholder", "0.0.0.0");
    expect(field).toHaveAccessibleDescription("Enter an IP address");

    const answers: [string, string][] = [
      ["10.1.2.3", "Access is allowed for this IP address"],
      [
        "10.2.0.1",
        "Access is not allowed for this IP address based on your Environment Inbound IP Rules.",
      ],
      [
        "192.168.1.1",
        "Access is not allowed for this IP address based on your Workspace Inbound IP Rules.",
      ],
      [
        "10.1.3.1",
        "Access is not allowed for this IP address based on your Web Service Inbound IP Rules.",
      ],
      ["10.1.2", "This IP address is not valid"],
      ["2001:db8::1", "This IP address is not valid"],
    ];
    for (const [address, answer] of answers) {
      await user.clear(field);
      await user.type(field, address);
      expect(field).toHaveAccessibleDescription(answer);
    }
    expect(field).toHaveAttribute("aria-invalid", "true");
  });

  it("checks the unsaved draft of the resource's own rules", async () => {
    levels(bothLevels);
    const user = userEvent.setup();
    const view = renderLevels();
    const field = screen.getByRole("textbox", { name: "Validate IP address" });
    await user.type(field, "10.1.3.1");
    expect(field).toHaveAccessibleDescription(/Web Service Inbound IP Rules/);

    view.rerender(
      <InboundIPLevels
        environmentId="evm-1"
        saved={service}
        draft={rules("10.1.0.0/16")}
        ownType="Web Service"
      />,
    );
    expect(field).toHaveAccessibleDescription(
      "Access is allowed for this IP address",
    );
    // The count stays on the saved rules, as Render's does.
    expect(screen.getByRole("status")).toHaveTextContent(/^1 IP range is/);
  });

  it("claims nothing while a level is being read, and says when one could not be", () => {
    levels({ state: "loading" });
    const view = renderLevels();
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
    // The workspace's and the environment's sections, at their ready place.
    expect(
      view.container.querySelectorAll(
        '[data-preview="inbound-ip-affected-by"]',
      ),
    ).toHaveLength(2);

    levels({ state: "error" });
    view.rerender(
      <InboundIPLevels saved={service} draft={service} ownType="Web Service" />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      /Couldn't load the workspace and environment inbound IP rules/,
    );
    expect(screen.queryByRole("status")).toBeNull();
  });
});

describe("InboundIPLevelsSkeleton", () => {
  // The skeleton is a structural preview (dashboard/AGENTS.md): the same
  // regions in the same order, each level's table at the seeded pair's two
  // rows, and the field's three lines.
  function regions(root: Element) {
    return [...root.children].map((child) => ({
      tag: child.tagName,
      className: child.className,
      children: child.children.length,
      rows: child.querySelectorAll("tr").length,
      inputs: child.querySelectorAll("input").length,
    }));
  }

  it.each([true, false])(
    "keeps the ready view's regions (environment: %s)",
    (inEnvironment) => {
      levels({
        state: "ready",
        workspace: ALLOW_ALL_PAIR_ENTRIES,
        environment: inEnvironment
          ? { entries: ALLOW_ALL_PAIR_ENTRIES, href: ENVIRONMENT_HREF }
          : undefined,
      });
      const ready = render(
        <InboundIPLevels
          environmentId={inEnvironment ? "evm-1" : null}
          saved={service}
          draft={service}
          ownType="Web Service"
        />,
      ).container.firstElementChild!;
      levels({ state: "loading" });
      const pending = render(
        <InboundIPLevels
          environmentId={inEnvironment ? "evm-1" : null}
          saved={service}
          draft={service}
          ownType="Web Service"
        />,
      ).container.firstElementChild!;

      expect(pending).toHaveAttribute("data-preview", "inbound-ip-levels");
      expect(pending.className).toBe(ready.className);
      const want = regions(ready);
      const got = regions(pending);
      expect(got).toHaveLength(want.length);
      got.forEach((region, index) => {
        // The skeleton may add positioning for its bars, never drop a class.
        for (const name of want[index].className.split(" ").filter(Boolean)) {
          expect(region.className.split(" ")).toContain(name);
        }
        expect(region.rows).toBe(want[index].rows);
        expect(region.inputs).toBe(want[index].inputs);
      });
      // The status line and the field, then the field's label, input and
      // message line.
      const [top] = [...pending.children];
      const [readyTop] = [...ready.children];
      expect(top.children[1].children).toHaveLength(
        readyTop.children[1].children.length,
      );
    },
  );
});

describe("every Networking panel mounts the levels", () => {
  beforeEach(() => {
    state.entries = rules("10.1.2.0/24");
    state.environmentId = "evm-1";
    levels(bothLevels);
  });

  it.each([
    {
      name: "a web service",
      ownType: "Web Service",
      panel: () => (
        <ServiceNetworkingPanel
          serviceId="srv-1"
          serviceType="web_service"
          environmentId="evm-1"
          currentAllowList={state.entries}
        />
      ),
    },
    {
      name: "a static site",
      ownType: "Static Site",
      panel: () => (
        <ServiceNetworkingPanel
          serviceId="srv-1"
          serviceType="static_site"
          environmentId="evm-1"
          currentAllowList={state.entries}
        />
      ),
    },
    {
      name: "a Postgres database",
      ownType: "PostgreSQL",
      panel: () => (
        <AccessControlPanel
          database={{
            id: "dpg-1",
            name: "db",
            plan: "basic_256mb",
            poolerEnabled: false,
            highAvailabilityEnabled: false,
          }}
          onChanged={() => undefined}
        />
      ),
    },
    {
      name: "a Key Value instance",
      ownType: "Key Value",
      panel: () => <KeyValueNetworkingPanel id="red-1" />,
    },
  ])(
    "$name reads its environment's level and names itself in a refusal",
    async ({ panel, ownType }) => {
      const user = userEvent.setup();
      render(panel());
      expect(vi.mocked(useInboundIPLevels)).toHaveBeenCalledWith("evm-1");
      expect(section("workspace")).toBeVisible();
      expect(section("environment")).toBeVisible();
      expect(
        screen.getByRole("heading", { name: `${ownType} Inbound IP Rules` }),
      ).toBeVisible();

      const field = screen.getByRole("textbox", {
        name: "Validate IP address",
      });
      await user.type(field, "10.1.3.1");
      expect(field).toHaveAccessibleDescription(
        `Access is not allowed for this IP address based on your ${ownType} Inbound IP Rules.`,
      );

      // Editing the own rules without saving changes the answer at once.
      const cidr = screen.getByRole("textbox", {
        name: "CIDR block for rule 1",
      });
      await user.clear(cidr);
      await user.type(cidr, "10.1.0.0/16");
      expect(field).toHaveAccessibleDescription(
        "Access is allowed for this IP address",
      );
    },
  );

  it("a service outside any environment reads no environment level", () => {
    levels({ state: "ready", workspace });
    render(
      <ServiceNetworkingPanel
        serviceId="srv-1"
        serviceType="web_service"
        environmentId={null}
        currentAllowList={state.entries}
      />,
    );
    expect(vi.mocked(useInboundIPLevels)).toHaveBeenCalledWith(null);
    expect(
      screen.queryByTestId("inbound-ip-affected-by-environment"),
    ).toBeNull();
  });

  it("an environment's card shows the workspace level only, and names the environment in a refusal", async () => {
    levels({ state: "ready", workspace });
    const user = userEvent.setup();
    render(
      <EnvironmentNetworkingCard
        environment={{
          id: "evm-1",
          projectId: "prj-1",
          name: "staging",
          ownerId: "tea-1",
          createdAt: null,
          serviceIds: [],
          databaseIds: [],
          keyValueIds: [],
          envGroupIds: [],
          protectedStatus: "unprotected",
          networkIsolationEnabled: false,
          ipAllowListEntries: rules("10.1.0.0/16"),
        }}
        belowScale={false}
      />,
    );
    expect(vi.mocked(useInboundIPLevels)).toHaveBeenCalledWith(undefined);
    expect(section("workspace")).toBeVisible();
    expect(
      screen.queryByTestId("inbound-ip-affected-by-environment"),
    ).toBeNull();
    await user.type(
      screen.getByRole("textbox", { name: "Validate IP address" }),
      "10.2.0.1",
    );
    expect(
      screen.getByRole("textbox", { name: "Validate IP address" }),
    ).toHaveAccessibleDescription(
      "Access is not allowed for this IP address based on your Environment Inbound IP Rules.",
    );
  });
});
