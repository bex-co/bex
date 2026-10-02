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
    users: [],
    creatingUser: false,
    pooled: null,
    poolLoading: false,
  }),
}));
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({ instanceTypes: [] }),
}));
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
    panel: () => <AccessControlPanel id="dpg-source" plan="free" />,
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
    it("can disable a legacy public endpoint that already has no IP rules", async () => {
      state.entries = [];
      const user = userEvent.setup();
      const view = render(panel());

      expect(screen.getByText(/External access remains enabled/)).toBeVisible();
      const save = screen.getByRole("button", { name: "Save allowlist" });
      expect(save).toBeEnabled();
      await user.click(save);
      expect(state.save).toHaveBeenCalledWith([]);

      // The server confirms that the same empty entries now mean private.
      state.isPublic = false;
      view.rerender(panel());
      expect(
        screen.getByText(/External connections are disabled/),
      ).toBeVisible();
      expect(screen.queryByText(/External access remains enabled/)).toBeNull();
      expect(save).toBeDisabled();
    });

    it("can enable an explicitly private endpoint with its existing IP rules", async () => {
      state.isPublic = false;
      const user = userEvent.setup();
      const view = render(panel());
      expect(
        screen.getByText(/External connections are disabled/),
      ).toBeVisible();
      await user.click(screen.getByRole("button", { name: "Save allowlist" }));
      expect(state.save).toHaveBeenCalledWith(state.entries);

      state.isPublic = true;
      view.rerender(panel());
      expect(
        screen.getByRole("button", { name: "Save allowlist" }),
      ).toBeDisabled();
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

it("keeps the service empty-list meaning and unchanged-save guard", () => {
  state.entries = [];
  render(serviceConsumer.panel());
  expect(screen.getByText("Open to all source IPs")).toBeVisible();
  expect(screen.getByRole("button", { name: /^Save/ })).toBeDisabled();
});
