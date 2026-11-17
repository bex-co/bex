import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KeyValuePlanSection } from "@/features/keyvalue/components/key-value-plan-section";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { KeyValueView } from "@/features/keyvalue/types";

const updatePlan = vi.fn();

const catalog = vi.hoisted(() => {
  const defaultTiers = [
    {
      id: "free",
      name: "Free",
      cpu: "0.1",
      memory: "256Mi",
      storageGB: 1,
      monthlyUsd: "0.00",
      connectionLimit: 50,
    },
    {
      id: "starter",
      name: "Starter",
      cpu: "0.5",
      memory: "1Gi",
      storageGB: 10,
      monthlyUsd: "7.00",
      connectionLimit: 250,
    },
  ];
  return { loading: false, instanceTypes: defaultTiers, defaultTiers };
});

vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () =>
    catalog.loading
      ? { instanceTypes: [], loading: true, error: undefined }
      : {
          instanceTypes: catalog.instanceTypes,
          loading: false,
          error: undefined,
        },
}));

vi.mock("@/features/keyvalue/hooks/use-update-key-value-plan", () => ({
  useUpdateKeyValuePlan: () => ({ updatePlan, busy: false }),
}));

const persistence = vi.hoisted(() => ({ mode: "journal-snapshot" }));

vi.mock("@/features/keyvalue/hooks/use-set-key-value-persistence-mode", () => ({
  useSetKeyValuePersistenceMode: () => ({
    mode: persistence.mode,
    loading: false,
    saving: false,
    save: vi.fn(),
  }),
}));

const KEY_VALUE: KeyValueView = {
  id: "red-sessions",
  name: "sessions",
  status: "available",
  plan: "free",
  version: "8",
  createdAt: null,
  externalHost: null,
  public: false,
  suspended: false,
};

beforeEach(() => {
  catalog.loading = false;
  catalog.instanceTypes = catalog.defaultTiers;
  persistence.mode = "journal-snapshot";
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  updatePlan.mockReset().mockResolvedValue(true);
});

describe("KeyValuePlanSection", () => {
  // w4/m208: while the catalog loads, the card keeps three tier slots in the
  // ready grid instead of an empty radio group.
  it("reserves a slot per catalog tier while the catalog loads", () => {
    catalog.loading = true;
    const { container } = render(
      <KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />,
    );
    expect(screen.queryByRole("radiogroup")).toBeNull();
    const slots = container.querySelector('[aria-hidden="true"]');
    expect(slots?.children).toHaveLength(9); // Render's ladder (w5/m266)
    // Each with the ready card's connection-limit line (w6/m459).
    for (const slot of Array.from(slots!.children)) {
      expect(slot.children).toHaveLength(3);
    }
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  // w6/m459: as on Render's plan cards, each option states its connection
  // limit, the current plan's included, read from the catalog.
  it("states every plan's connection limit, the current one's too", () => {
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />);
    const free = screen.getByRole("radio", { name: /Free/ });
    expect(free).toHaveAttribute("aria-checked", "true");
    expect(free).toHaveTextContent("50 connections");
    expect(screen.getByRole("radio", { name: /Starter/ })).toHaveTextContent(
      "250 connections",
    );
  });

  // w8/249: Render does not downgrade a Key Value instance, and bex-api
  // refuses it. A smaller plan's card stays in place, disabled, with the reason.
  it("disables every plan smaller than the current one", async () => {
    const user = userEvent.setup();
    render(
      <KeyValuePlanSection
        keyValue={{ ...KEY_VALUE, plan: "starter" }}
        onChanged={vi.fn()}
      />,
    );

    const free = screen.getByRole("radio", { name: /Free/ });
    expect(free).toBeDisabled();
    expect(free).toHaveTextContent("Unavailable");
    expect(free).toHaveAttribute(
      "title",
      expect.stringContaining("can't move to a smaller instance type"),
    );
    const starter = screen.getByRole("radio", { name: /Starter/ });
    expect(starter).toBeEnabled();
    expect(starter).toHaveAttribute("aria-checked", "true");
    await user.click(free);
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(updatePlan).not.toHaveBeenCalled();
  });

  // The smaller-plan rule compares the catalog's memory (bex-api's
  // keyvalue.ErrPlanDowngrade), so a catalog that is not ordered
  // smallest-first still disables exactly the smaller tiers.
  it("judges plan size by memory, not by the catalog's order", () => {
    catalog.instanceTypes = [...catalog.defaultTiers].reverse();
    render(
      <KeyValuePlanSection
        keyValue={{ ...KEY_VALUE, plan: "starter" }}
        onChanged={vi.fn()}
      />,
    );

    expect(screen.getByRole("radio", { name: /Free/ })).toBeDisabled();
    expect(screen.getByRole("radio", { name: /Starter/ })).toBeEnabled();
  });

  it("leaves every plan selectable on the smallest plan", () => {
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />);
    expect(screen.getByRole("radio", { name: /Free/ })).toBeEnabled();
    expect(screen.getByRole("radio", { name: /Starter/ })).toBeEnabled();
  });

  it("lets a contributor change plan through can_operate", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={onChanged} />);

    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(updatePlan).toHaveBeenCalledWith(
      "red-sessions",
      "starter",
      "Starter",
    );
    expect(onChanged).toHaveBeenCalledOnce();
  });

  // w4/156: a free store became a paid one in two clicks with no cost shown.
  it("prices every plan card and states the new price before the change", async () => {
    const user = userEvent.setup();
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />);

    expect(screen.getByRole("radio", { name: /Starter/ })).toHaveTextContent(
      "$7.00/month",
    );
    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("alertdialog")).toHaveTextContent(
      "The new plan costs $7.00/month.",
    );
  });

  it("disables plan controls for a viewer with the operate reason", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canOperate: false }),
    );
    const user = userEvent.setup();
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />);

    const starter = screen.getByRole("radio", { name: /Starter/ });
    expect(starter).toBeDisabled();
    expect(
      screen.getByText(/Your role can only view this service/),
    ).toBeInTheDocument();
    await user.click(starter);
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("fail-closes plan controls until capabilities are definitive", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canOperate: false, loading: true, loaded: false }),
    );
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />);
    expect(screen.getByRole("radio", { name: /Starter/ })).toBeDisabled();
  });

  // w4/m263: the restart is real, and it empties a Persistence Off store.
  it("warns of the restart without a phrase on a persisted store", async () => {
    const user = userEvent.setup();
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />);
    expect(document.body).not.toHaveTextContent(
      /operator|reconcile|preserves data/i,
    );

    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("restarts");
    expect(dialog).not.toHaveTextContent("deletes every key");
    expect(within(dialog).queryByRole("textbox")).toBeNull();
  });

  it("requires the typed phrase before a Persistence Off store restarts", async () => {
    persistence.mode = "off";
    const user = userEvent.setup();
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />);

    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("deletes every key in sessions");
    const confirm = within(dialog).getByRole("button", { name: "Save" });
    expect(confirm).toBeDisabled();
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo clear key value sessions",
    );
    expect(confirm).toBeEnabled();
    await user.click(confirm);
    expect(updatePlan).toHaveBeenCalledWith(
      "red-sessions",
      "starter",
      "Starter",
    );
  });

  it("sends nothing when the Off confirmation is cancelled", async () => {
    persistence.mode = "off";
    const user = userEvent.setup();
    render(<KeyValuePlanSection keyValue={KEY_VALUE} onChanged={vi.fn()} />);
    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(updatePlan).not.toHaveBeenCalled();
  });
});
