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

async function openAddWith(name: string) {
  await userEvent.click(
    screen.getByRole("button", { name: "Add read replica" }),
  );
  await userEvent.type(screen.getByLabelText("Name"), name);
}

describe("HAPanel read replicas (w5/m138)", () => {
  it("adds a replica by sending the whole set, without a racing refetch", async () => {
    const refetch = vi.fn();
    render(<HAPanel database={WITH_REPLICA} refetch={refetch} />);
    await openAddWith("r2");
    await userEvent.click(screen.getByRole("button", { name: "Add replica" }));

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "dpg-shop",
        readReplicas: [{ name: "r1" }, { name: "r2" }],
      },
    });
    // The mutation answers the new set into the cache; a refetch would
    // race the operator and could bring back the old set.
    expect(refetch).not.toHaveBeenCalled();
  });

  it("refuses a name bex-api would refuse before calling it", async () => {
    render(<HAPanel database={WITH_REPLICA} refetch={vi.fn()} />);
    await openAddWith("Reader_2");
    expect(
      screen.getByText(
        "Use 1–35 lowercase letters, digits or hyphens, starting and ending with a letter or digit.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add replica" })).toBeDisabled();

    await userEvent.clear(screen.getByLabelText("Name"));
    await userEvent.type(screen.getByLabelText("Name"), "r1");
    expect(
      screen.getByText("This database already has a read replica named r1."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add replica" })).toBeDisabled();
    expect(mutate).not.toHaveBeenCalled();
  });

  it("keeps the dialog open with the name when the API refuses", async () => {
    mutate.mockRejectedValue(new Error("read replicas require more storage"));
    render(<HAPanel database={WITH_REPLICA} refetch={vi.fn()} />);
    await openAddWith("r2");
    await userEvent.click(screen.getByRole("button", { name: "Add replica" }));
    expect(screen.getByLabelText("Name")).toHaveValue("r2");
  });

  it("removes a replica only after confirmation", async () => {
    render(<HAPanel database={WITH_REPLICA} refetch={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: "Remove" }));
    expect(mutate).not.toHaveBeenCalled();
    expect(screen.getByText("Remove read replica r1?")).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: "Remove replica" }),
    );
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "dpg-shop", readReplicas: [] },
    });
  });

  it("disables adding on a plan without replicas, at the limit, and below the storage minimum", () => {
    const { rerender } = render(
      <HAPanel database={DATABASE} refetch={vi.fn()} />,
    );
    const add = () => screen.getByRole("button", { name: "Add read replica" });
    expect(add()).toBeDisabled();

    rerender(
      <HAPanel
        database={{
          ...WITH_REPLICA,
          readReplicas: [
            ...WITH_REPLICA.readReplicas,
            { name: "r2", connectionInfo: null },
          ],
        }}
        refetch={vi.fn()}
      />,
    );
    expect(add()).toBeDisabled();

    rerender(
      <HAPanel
        database={{ ...WITH_REPLICA, diskSizeGB: 5 }}
        refetch={vi.fn()}
      />,
    );
    expect(add()).toBeDisabled();

    rerender(<HAPanel database={WITH_REPLICA} refetch={vi.fn()} />);
    expect(add()).toBeEnabled();
  });

  it("disables add and remove for a member who cannot operate the database", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        canOperate: false,
        reasonKey: (action) =>
          action === "can_operate" ? "databases.haFailover" : undefined,
      }),
    );
    render(<HAPanel database={WITH_REPLICA} refetch={vi.fn()} />);
    expect(
      screen.getByRole("button", { name: "Add read replica" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Remove" })).toBeDisabled();
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

  it("disables the switch for a member who cannot operate the database", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        canOperate: false,
        reasonKey: (action) =>
          action === "can_operate" ? "databases.haFailover" : undefined,
      }),
    );
    render(
      <HAPanel database={{ ...DATABASE, plan: "pro" }} refetch={vi.fn()} />,
    );
    expect(haSwitch()).toBeDisabled();
  });
});

describe("HAPanel pending preview (w5/m138)", () => {
  function replicasHeader(card: Element): Element {
    const label = [...card.querySelectorAll("p")].find(
      (p) => p.textContent === "Read replicas",
    );
    expect(label).toBeDefined();
    return label!.parentElement!;
  }

  it("has the ready card's header and Read replicas row, inert and with Add disabled", () => {
    const pending = render(<DatabaseHighAvailabilityPreview />).container
      .firstElementChild!;
    const ready = render(<HAPanel database={WITH_REPLICA} refetch={vi.fn()} />)
      .container.firstElementChild!;

    const header = (card: Element) =>
      card.querySelector('[data-slot="card-header"]')!;
    expect(shape(header(pending))).toBe(shape(header(ready)));
    expect(shape(replicasHeader(pending))).toBe(shape(replicasHeader(ready)));

    expect(pending.hasAttribute("inert")).toBe(true);
    expect(
      replicasHeader(pending).querySelector("button")!.hasAttribute("disabled"),
    ).toBe(true);
    // No replica data before it exists.
    expect(pending.textContent).not.toContain("r1");
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
