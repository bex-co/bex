import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccessControlPanel } from "@/features/databases/components/access-control-panel";
import { KeyValueNetworkingPanel } from "@/features/keyvalue/components/key-value-networking-panel";
import { ServiceNetworkingPanel } from "@/features/services/components/service-networking-panel";

const state = vi.hoisted(() => ({
  isPublic: true,
  entries: [{ cidrBlock: "203.0.113.0/24", description: "office" }],
  save: vi.fn(async (..._args: unknown[]) => true),
}));

vi.mock("@/features/databases/hooks/use-access-control", () => ({
  useAccessControl: () => ({
    allowList: state.entries,
    isPublic: state.isPublic,
    savingAllowList: false,
    saveAllowList: state.save,
    credentials: [],
    creatingCredential: false,
    pooled: null,
    poolLoading: false,
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
    isPublic: state.isPublic,
    savingAllowList: false,
    saveAllowList: state.save,
  }),
}));
vi.mock("@/features/services/hooks/use-service-networking", () => ({
  useServiceNetworking: () => ({ saving: false, saveAllowList: state.save }),
}));

const datastoreConsumers = [
  {
    name: "Postgres",
    panel: () => (
      <AccessControlPanel
        database={{
          id: "dpg-source",
          name: "source",
          plan: "free",
          poolerEnabled: false,
          highAvailabilityEnabled: false,
        }}
        onChanged={() => undefined}
      />
    ),
  },
  {
    name: "Key Value",
    panel: () => <KeyValueNetworkingPanel id="kv-source" />,
  },
];
const serviceConsumer = {
  name: "service",
  panel: () => (
    <ServiceNetworkingPanel
      serviceId="srv-source"
      currentAllowList={state.entries}
    />
  ),
};
const consumers = [...datastoreConsumers, serviceConsumer];

beforeEach(() => {
  state.isPublic = true;
  state.entries = [{ cidrBlock: "203.0.113.0/24", description: "office" }];
  state.save.mockClear();
});

describe.each(consumers)("$name IP allowlist draft", ({ name, panel }) => {
  it("keeps focus and unsaved edits across equivalent refreshes, resets on authoritative changes, and saves plain entries", async () => {
    const user = userEvent.setup();
    const view = render(panel());
    const cidr = screen.getByRole("textbox", { name: "CIDR block for rule 1" });
    await user.clear(cidr);
    await user.type(cidr, "10.0.0.0/8");
    expect(cidr).toHaveValue("10.0.0.0/8");
    expect(cidr).toHaveFocus();
    expect(state.save).not.toHaveBeenCalled();

    state.entries = state.entries.map((entry) => ({ ...entry }));
    view.rerender(panel());
    expect(screen.getByRole("textbox", { name: "CIDR block for rule 1" })).toBe(
      cidr,
    );
    expect(cidr).toHaveValue("10.0.0.0/8");
    expect(cidr).toHaveFocus();
    expect(screen.getByRole("button", { name: /^Save/ })).toBeEnabled();

    state.entries = [
      { cidrBlock: "192.0.2.0/24", description: "remote office" },
    ];
    view.rerender(panel());
    expect(
      screen.getByRole("textbox", { name: "CIDR block for rule 1" }),
    ).toHaveValue("192.0.2.0/24");
    expect(
      screen.getByRole("textbox", { name: "Description for rule 1" }),
    ).toHaveValue("remote office");
    expect(screen.getByRole("button", { name: /^Save/ })).toBeDisabled();
    expect(state.save).not.toHaveBeenCalled();

    const updatedCIDR = screen.getByRole("textbox", {
      name: "CIDR block for rule 1",
    });
    await user.clear(updatedCIDR);
    await user.type(updatedCIDR, "198.51.100.0/24");
    expect(updatedCIDR).toHaveFocus();
    expect(state.save).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: /^Save/ }));
    const expected = [
      { cidrBlock: "198.51.100.0/24", description: "remote office" },
    ];
    expect(state.save.mock.calls).toEqual([
      name === "service" ? ["srv-source", expected] : [expected],
    ]);
  });
});

describe.each(datastoreConsumers)(
  "$name datastore external access",
  ({ panel }) => {
    // w4/m315: bex-api reads a public store with no saved rules as the
    // unrestricted pair, Render's own spelling of open access.
    it("lists a public store's unrestricted access as rule rows, and clearing them disables it", async () => {
      state.entries = [
        { cidrBlock: "0.0.0.0/0", description: "" },
        { cidrBlock: "::/0", description: "" },
      ];
      const user = userEvent.setup();
      render(panel());
      expect(
        screen.getByRole("textbox", { name: "CIDR block for rule 1" }),
      ).toHaveValue("0.0.0.0/0");
      expect(
        screen.getByRole("textbox", { name: "CIDR block for rule 2" }),
      ).toHaveValue("::/0");
      expect(
        screen.queryByText(/External connections are disabled/),
      ).toBeNull();
      expect(screen.queryByText(/No IP rules/)).toBeNull();
      const save = screen.getByRole("button", { name: "Save allowlist" });
      expect(save).toBeDisabled();

      await user.click(
        screen.getByRole("button", { name: "Remove 0.0.0.0/0" }),
      );
      await user.click(screen.getByRole("button", { name: "Remove ::/0" }));
      await user.click(save);
      expect(state.save).toHaveBeenCalledWith([]);
    });

    it("reads a private store as disabled with no rules, and saving a rule enables it", async () => {
      state.isPublic = false;
      state.entries = [];
      const user = userEvent.setup();
      render(panel());
      expect(
        screen.getByText(/External connections are disabled/),
      ).toBeVisible();
      expect(
        screen.getByText(
          "No IP rules. Save an empty list to block external connections.",
        ),
      ).toBeVisible();
      const save = screen.getByRole("button", { name: "Save allowlist" });
      expect(save).toBeDisabled();

      await user.type(
        screen.getByRole("textbox", { name: "New CIDR block" }),
        "203.0.113.0/24",
      );
      await user.click(screen.getByRole("button", { name: "Add" }));
      await user.click(save);
      expect(state.save).toHaveBeenCalledWith([
        { cidrBlock: "203.0.113.0/24", description: "" },
      ]);
    });

    it("explains and submits external disable when the last rule is removed", async () => {
      const user = userEvent.setup();
      render(panel());
      await user.click(
        screen.getByRole("button", { name: "Remove 203.0.113.0/24" }),
      );
      expect(
        screen.getByText(
          "No IP rules. Save an empty list to block external connections.",
        ),
      ).toBeVisible();
      expect(screen.queryByText("Open to all source IPs.")).toBeNull();
      await user.click(screen.getByRole("button", { name: "Save allowlist" }));
      expect(state.save).toHaveBeenCalledWith([]);
    });
  },
);

// w8/m157: a service with no rule is blocked, as on Render, not open.
it("reads a service with no rule as blocked and keeps the unchanged-save guard", () => {
  state.entries = [];
  render(serviceConsumer.panel());
  expect(
    screen.getByText(/all inbound traffic to this service is blocked/),
  ).toBeVisible();
  expect(screen.queryByText("Open to all source IPs")).toBeNull();
  expect(screen.getByRole("button", { name: /^Save/ })).toBeDisabled();
});
