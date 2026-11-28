import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabaseReadReplicas } from "@/features/databases/components/database-read-replicas";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
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

async function openAddWith(name: string) {
  await userEvent.click(
    screen.getByRole("button", { name: "Add read replica" }),
  );
  await userEvent.type(screen.getByLabelText("Name"), name);
}

// The Details card's read-replica field (w3/116; moved from the HA panel,
// w5/m138).
describe("DatabaseReadReplicas", () => {
  it("lists each replica with its internal host, or says there are none", () => {
    const { rerender } = render(
      <DatabaseReadReplicas database={WITH_REPLICA} />,
    );
    expect(screen.getByText("Read replicas")).toBeInTheDocument();
    expect(screen.getByText("r1")).toBeInTheDocument();
    expect(screen.getByText("dpg-shop-ro-r1")).toBeInTheDocument();
    expect(screen.queryByText("No named read replicas.")).toBeNull();

    rerender(<DatabaseReadReplicas database={DATABASE} />);
    expect(screen.getByText("No named read replicas.")).toBeInTheDocument();
  });

  it("adds a replica by sending the whole set", async () => {
    render(<DatabaseReadReplicas database={WITH_REPLICA} />);
    await openAddWith("r2");
    await userEvent.click(screen.getByRole("button", { name: "Add replica" }));

    // The mutation answers the new set into the cache; nothing refetches, so
    // nothing races the operator back to the old set.
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "dpg-shop",
        readReplicas: [{ name: "r1" }, { name: "r2" }],
      },
    });
  });

  it("refuses a name bex-api would refuse before calling it", async () => {
    render(<DatabaseReadReplicas database={WITH_REPLICA} />);
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
    render(<DatabaseReadReplicas database={WITH_REPLICA} />);
    await openAddWith("r2");
    await userEvent.click(screen.getByRole("button", { name: "Add replica" }));
    expect(screen.getByLabelText("Name")).toHaveValue("r2");
  });

  it("removes a replica only after confirmation", async () => {
    render(<DatabaseReadReplicas database={WITH_REPLICA} />);
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
    const { rerender } = render(<DatabaseReadReplicas database={DATABASE} />);
    const add = () => screen.getByRole("button", { name: "Add read replica" });
    expect(add()).toBeDisabled();

    rerender(
      <DatabaseReadReplicas
        database={{
          ...WITH_REPLICA,
          readReplicas: [
            ...WITH_REPLICA.readReplicas,
            { name: "r2", connectionInfo: null },
          ],
        }}
      />,
    );
    expect(add()).toBeDisabled();

    rerender(
      <DatabaseReadReplicas database={{ ...WITH_REPLICA, diskSizeGB: 5 }} />,
    );
    expect(add()).toBeDisabled();

    rerender(<DatabaseReadReplicas database={WITH_REPLICA} />);
    expect(add()).toBeEnabled();
  });

  // Read replicas are a setting PATCH /v1/postgres changes: can_create, so a
  // Contributor, who keeps can_operate, is held (w9/m619).
  it("disables add and remove for a contributor, and keeps them for a developer", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const { rerender } = render(
      <DatabaseReadReplicas database={WITH_REPLICA} />,
    );
    expect(
      screen.getByRole("button", { name: "Add read replica" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Remove" })).toBeDisabled();

    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    rerender(<DatabaseReadReplicas database={WITH_REPLICA} />);
    expect(
      screen.getByRole("button", { name: "Add read replica" }),
    ).toBeEnabled();
    expect(screen.getByRole("button", { name: "Remove" })).toBeEnabled();
  });

  // Fail-closed: no answer yet is no grant.
  it("holds add and remove while capabilities are still checking", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        canOperate: false,
        canCreate: false,
        loading: true,
        loaded: false,
      }),
    );
    render(<DatabaseReadReplicas database={WITH_REPLICA} />);
    expect(
      screen.getByRole("button", { name: "Add read replica" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Remove" })).toBeDisabled();
  });
});
