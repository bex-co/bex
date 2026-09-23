import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccessControlPanel } from "@/features/databases/components/access-control-panel";
import { KeyValueNetworkingPanel } from "@/features/keyvalue/components/key-value-networking-panel";
import { ServiceNetworkingPanel } from "@/features/services/components/service-networking-panel";

const state = vi.hoisted(() => ({
  entries: [{ cidrBlock: "203.0.113.0/24", description: "office" }],
  save: vi.fn(async (..._args: unknown[]) => true),
}));

vi.mock("@/features/databases/hooks/use-access-control", () => ({
  useAccessControl: () => ({
    allowList: state.entries,
    savingAllowList: false,
    saveAllowList: state.save,
    users: [],
    creatingUser: false,
    pooled: null,
    poolLoading: false,
  }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value-networking", () => ({
  useKeyValueNetworking: () => ({
    allowList: state.entries,
    savingAllowList: false,
    saveAllowList: state.save,
  }),
}));
vi.mock("@/features/services/hooks/use-service-networking", () => ({
  useServiceNetworking: () => ({ saving: false, saveAllowList: state.save }),
}));

const consumers = [
  { name: "Postgres", panel: () => <AccessControlPanel id="dpg-source" /> },
  {
    name: "Key Value",
    panel: () => <KeyValueNetworkingPanel id="kv-source" isPublic />,
  },
  {
    name: "service",
    panel: () => (
      <ServiceNetworkingPanel
        serviceId="srv-source"
        currentAllowList={state.entries}
      />
    ),
  },
];

beforeEach(() => {
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
