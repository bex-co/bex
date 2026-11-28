import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HAPanel } from "@/features/databases/components/ha-panel";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { shape } from "@/test/card-shape";
import { DatabaseHighAvailabilityPreview } from "@/common/components/datastore-overview-previews";
import type { DatabaseDetailView } from "@/features/databases/types";

const mutate = vi.hoisted(() => vi.fn());

vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate, { loading: false }],
}));

vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes: [
      {
        id: "free",
        name: "Free",
        cpu: "100m",
        memory: "256Mi",
        storageGB: 1,
        maxReadReplicas: 0,
        readReplicaMinStorageGB: 10,
        supportsHighAvailability: false,
      },
      {
        id: "basic-1gb",
        name: "Basic 1 GB",
        cpu: "500m",
        memory: "1Gi",
        storageGB: 5,
        maxReadReplicas: 2,
        readReplicaMinStorageGB: 10,
        supportsHighAvailability: false,
      },
      {
        id: "pro",
        name: "Pro",
        cpu: "1",
        memory: "4Gi",
        storageGB: 20,
        maxReadReplicas: 5,
        readReplicaMinStorageGB: 10,
        supportsHighAvailability: true,
        monthlyUsd: "38.50",
      },
    ],
    loading: false,
    error: undefined,
  }),
}));

const DATABASE: DatabaseDetailView = {
  id: "dpg-shop",
  name: "shop",
  status: "available",
  plan: "free",
  version: "18",
  diskSizeGB: 1,
  createdAt: null,
  public: false,
  suspended: "not_suspended",
  databaseName: "shop",
  databaseUser: "shop_user",
  highAvailabilityEnabled: false,
  diskAutoscalingEnabled: false,
  poolerEnabled: false,
  readReplicas: [],
  externalHost: null,
  backupsEnabled: false,
};

const WITH_REPLICA: DatabaseDetailView = {
  ...DATABASE,
  plan: "basic-1gb",
  diskSizeGB: 10,
  readReplicas: [
    {
      name: "r1",
      connectionInfo: {
        internalHost: "dpg-shop-ro-r1",
        externalHost: null,
      },
    },
  ],
};

beforeEach(() => {
  mutate.mockReset();
  mutate.mockResolvedValue({ data: {} });
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

describe("HAPanel plan requirement (w8/m43)", () => {
  it("names the 1-CPU requirement on a plan without HA", () => {
    render(<HAPanel database={DATABASE} refetch={vi.fn()} />);
    expect(
      screen.getByText(
        "High availability requires a plan with at least 1 CPU. This database's plan does not offer it.",
      ),
    ).toBeInTheDocument();
  });

  it("keeps the neutral line on a plan that offers HA", () => {
    render(
      <HAPanel database={{ ...DATABASE, plan: "pro" }} refetch={vi.fn()} />,
    );
    expect(
      screen.getByText("High availability is not enabled for this database."),
    ).toBeInTheDocument();
  });
});

// w3/116: the read replicas are a field of the Info page's Details card, as
// Render's General card lists them; its High Availability card has none.
describe("HAPanel without read replicas (w3/116)", () => {
  it("keeps only HA and failover, even for a database with replicas", () => {
    render(
      <HAPanel
        database={{
          ...WITH_REPLICA,
          plan: "pro",
          highAvailabilityRequested: true,
          highAvailabilityEnabled: true,
        }}
        refetch={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("switch", { name: "High availability" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Trigger failover" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Read replicas")).toBeNull();
    expect(screen.queryByText("r1")).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Add read replica" }),
    ).toBeNull();
  });
});

// w2/m171: the HA switch. It shows the spec intent, offers enabling only on a
// plan the catalog marks as supporting HA (bex-api's 1-CPU rule), always
// allows turning it off, and asks before either.
describe("HAPanel high availability switch (w2/m171)", () => {
  const haSwitch = () =>
    screen.getByRole("switch", { name: "High availability" });

  it("enables HA on a plan that offers it, after confirmation", async () => {
    const refetch = vi.fn();
    render(
      <HAPanel database={{ ...DATABASE, plan: "pro" }} refetch={refetch} />,
    );
    expect(haSwitch()).toBeEnabled();
    expect(haSwitch()).not.toBeChecked();
    await userEvent.click(haSwitch());
    expect(mutate).not.toHaveBeenCalled();
    expect(
      screen.getByText("Enable high availability on shop?"),
    ).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: "Enable high availability" }),
    );
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "dpg-shop", enabled: true },
    });
    expect(refetch).toHaveBeenCalled();
  });

  it("keeps the switch off and disabled on a plan below 1 CPU, and says why", () => {
    render(<HAPanel database={DATABASE} refetch={vi.fn()} />);
    expect(haSwitch()).toBeDisabled();
    expect(haSwitch()).not.toBeChecked();
    expect(
      screen.getByText(
        "High availability requires a plan with at least 1 CPU. This database's plan does not offer it.",
      ),
    ).toBeInTheDocument();
  });

  it("shows a requested standby still starting, and turns HA off", async () => {
    render(
      <HAPanel
        database={{
          ...DATABASE,
          plan: "pro",
          highAvailabilityRequested: true,
          highAvailabilityEnabled: false,
        }}
        refetch={vi.fn()}
      />,
    );
    expect(haSwitch()).toBeChecked();
    expect(
      screen.getByText(
        "A standby is starting on another node. Failover is available once it is ready.",
      ),
    ).toBeInTheDocument();
    await userEvent.click(haSwitch());
    await userEvent.click(
      screen.getByRole("button", { name: "Disable high availability" }),
    );
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "dpg-shop", enabled: false },
    });
  });

  it("lets HA left on a plan below 1 CPU be turned off", () => {
    render(
      <HAPanel
        database={{
          ...DATABASE,
          highAvailabilityRequested: true,
          highAvailabilityEnabled: true,
        }}
        refetch={vi.fn()}
      />,
    );
    expect(haSwitch()).toBeChecked();
    expect(haSwitch()).toBeEnabled();
  });

  // The HA switch is a setting (can_create) while failover is a lifecycle
  // verb a Contributor keeps (can_operate), as Render's role table has it
  // (w9/m619).
  it("holds the switch for a contributor but keeps failover enabled", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const live = {
      ...DATABASE,
      plan: "pro",
      highAvailabilityRequested: true,
      highAvailabilityEnabled: true,
    };
    render(<HAPanel database={live} refetch={vi.fn()} />);
    expect(haSwitch()).toBeDisabled();
    expect(haSwitch().closest('[data-slot="tooltip-trigger"]')).not.toBeNull();
    const failover = screen.getByRole("button", { name: "Trigger failover" });
    expect(failover).toBeEnabled();
    await userEvent.click(failover);
    expect(screen.getByText("Trigger failover on shop?")).toBeInTheDocument();
  });

  it("leaves the switch and failover enabled for a developer", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    render(
      <HAPanel
        database={{
          ...DATABASE,
          plan: "pro",
          highAvailabilityRequested: true,
          highAvailabilityEnabled: true,
        }}
        refetch={vi.fn()}
      />,
    );
    expect(haSwitch()).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Trigger failover" }),
    ).toBeEnabled();
  });

  it("holds failover for a viewer without can_operate", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canOperate: false, canCreate: false }),
    );
    render(
      <HAPanel
        database={{
          ...DATABASE,
          plan: "pro",
          highAvailabilityRequested: true,
          highAvailabilityEnabled: true,
        }}
        refetch={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Trigger failover" }),
    ).toBeDisabled();
  });

  // The verb refuses failover on a suspended database (409 since w32/111),
  // so the panel holds its button the way restart holds its own: an enabled
  // button must never promise a verb the server refuses on rendered state.
  it("holds failover while the database is suspended", () => {
    render(
      <HAPanel
        database={{
          ...DATABASE,
          plan: "pro",
          highAvailabilityRequested: true,
          highAvailabilityEnabled: true,
          suspended: "suspended",
        }}
        refetch={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Trigger failover" }),
    ).toBeDisabled();
    expect(mutate).not.toHaveBeenCalled();
  });

  // Fail-closed: no answer yet is no grant, so every control is held, and
  // none claims a role reason it has not been given.
  it("holds the switch and failover while capabilities are still checking", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        canOperate: false,
        canCreate: false,
        loading: true,
        loaded: false,
      }),
    );
    render(
      <HAPanel
        database={{
          ...WITH_REPLICA,
          plan: "pro",
          highAvailabilityRequested: true,
          highAvailabilityEnabled: true,
        }}
        refetch={vi.fn()}
      />,
    );
    expect(haSwitch()).toBeDisabled();
    expect(haSwitch().closest('[data-slot="tooltip-trigger"]')).toBeNull();
    expect(
      screen.getByRole("button", { name: "Trigger failover" }),
    ).toBeDisabled();
  });

  // Render bills the standby like the primary, so the confirmation names
  // what enabling adds before the user commits to it.
  it("names the standby's monthly price before enabling", async () => {
    render(
      <HAPanel database={{ ...DATABASE, plan: "pro" }} refetch={vi.fn()} />,
    );
    await userEvent.click(haSwitch());
    expect(
      screen.getByText(/The standby adds \$38\.50\/month\./),
    ).toBeInTheDocument();
  });

  it("keeps the confirmation open and does not refetch when the API refuses", async () => {
    mutate.mockRejectedValueOnce(new Error("refused"));
    const refetch = vi.fn();
    render(
      <HAPanel database={{ ...DATABASE, plan: "pro" }} refetch={refetch} />,
    );
    await userEvent.click(haSwitch());
    await userEvent.click(
      screen.getByRole("button", { name: "Enable high availability" }),
    );
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(refetch).not.toHaveBeenCalled();
    expect(
      screen.getByRole("button", { name: "Enable high availability" }),
    ).toBeInTheDocument();
  });

  // Turned off while the standby still runs: the switch shows the request
  // (off), the badge the observed standby (on), and the line explains the gap.
  // Re-enabling stays possible on a plan that offers HA.
  it("shows a standby still being removed", () => {
    render(
      <HAPanel
        database={{
          ...DATABASE,
          plan: "pro",
          highAvailabilityRequested: false,
          highAvailabilityEnabled: true,
        }}
        refetch={vi.fn()}
      />,
    );
    expect(haSwitch()).not.toBeChecked();
    expect(haSwitch()).toBeEnabled();
    expect(
      screen.getByText(
        "High availability is turning off. The standby is being removed.",
      ),
    ).toBeInTheDocument();
  });
});

describe("HAPanel pending preview (w5/m138)", () => {
  // The card's header and content box, inert, and no read-replica row in
  // either state (w3/116): the replicas left the card for the Details card.
  it("has the ready card's header, inert and without read replicas", () => {
    const pending = render(<DatabaseHighAvailabilityPreview />).container
      .firstElementChild!;
    const ready = render(<HAPanel database={WITH_REPLICA} refetch={vi.fn()} />)
      .container.firstElementChild!;

    const header = (card: Element) =>
      card.querySelector('[data-slot="card-header"]')!;
    expect(shape(header(pending))).toBe(shape(header(ready)));
    const content = (card: Element) =>
      card.querySelector('[data-slot="card-content"]')!;
    expect(content(pending).className).toBe(content(ready).className);
    // The status row and the plan line, in both.
    expect(content(pending).children).toHaveLength(
      content(ready).children.length,
    );

    expect(pending.hasAttribute("inert")).toBe(true);
    expect(pending.textContent).not.toContain("Read replicas");
    expect(ready.textContent).not.toContain("Read replicas");
  });

  // w2/m171: the status row carries the HA switch on the right in both
  // states — the same row and right-hand group, a switch-sized slot pending.
  it("previews the status row's switch group", () => {
    const pending = render(<DatabaseHighAvailabilityPreview />).container
      .firstElementChild!;
    const ready = render(<HAPanel database={DATABASE} refetch={vi.fn()} />)
      .container.firstElementChild!;
    const statusRow = (card: Element) =>
      card.querySelector('[data-slot="card-content"]')!.firstElementChild!;
    expect(statusRow(pending).className).toBe(statusRow(ready).className);
    const [, pendingGroup] = [...statusRow(pending).children];
    const [, readyGroup] = [...statusRow(ready).children];
    expect(pendingGroup.className).toBe(readyGroup.className);
    expect(readyGroup.querySelector('[role="switch"]')!.className).toContain(
      "h-5 w-9",
    );
    expect(pendingGroup.firstElementChild!.className).toContain("h-5 w-9");
  });
});
