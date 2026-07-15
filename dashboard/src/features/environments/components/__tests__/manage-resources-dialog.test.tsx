import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ManageResourcesDialog } from "@/features/environments/components/manage-resources-dialog";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { ServiceView } from "@/features/services/types";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";

const setServices = vi.fn();
vi.mock("@/features/environments/hooks/use-set-environment-services", () => ({
  useSetEnvironmentServices: () => ({ setServices, busyId: null }),
}));

const setDatabases = vi.fn();
vi.mock("@/features/environments/hooks/use-set-environment-databases", () => ({
  useSetEnvironmentDatabases: () => ({ setDatabases, busyId: null }),
}));

const setKeyValues = vi.fn();
vi.mock("@/features/environments/hooks/use-set-environment-keyvalues", () => ({
  useSetEnvironmentKeyValues: () => ({ setKeyValues, busyId: null }),
}));

const setEnvGroups = vi.fn();
vi.mock("@/features/environments/hooks/use-set-environment-env-groups", () => ({
  useSetEnvironmentEnvGroups: () => ({ setEnvGroups, busyId: null }),
}));

vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({
    groups: [
      {
        id: "evg-shared",
        name: "shared",
        serviceLinks: [],
        envVarKeys: [],
        secretFileNames: [],
      },
      {
        id: "evg-production",
        name: "production-secrets",
        serviceLinks: [],
        envVarKeys: [],
        secretFileNames: [],
      },
    ],
    loading: false,
  }),
}));

function svc(id: string): ServiceView {
  return {
    id,
    name: id,
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: null,
    createdAt: null,
    replicas: 1,
    revision: "r1",
    plan: null,
    idleTTLSeconds: null,
    schedule: null,
    command: null,
  } as ServiceView;
}

function db(id: string): DatabaseView {
  return {
    id,
    name: id,
    status: "available",
    plan: null,
    version: null,
    diskSizeGB: null,
    createdAt: null,
    public: false,
    suspended: "not_suspended",
  };
}

function kv(id: string): KeyValueView {
  return {
    id,
    name: id,
    status: "available",
    plan: null,
    version: null,
    createdAt: null,
    externalHost: null,
    public: false,
    suspended: false,
  };
}

const env: EnvironmentView = {
  id: "env-1",
  projectId: "prj-1",
  name: "staging",
  ownerId: "tea-1",
  createdAt: null,
  serviceIds: ["api"],
  databaseIds: ["primary-db"],
  keyValueIds: ["cache"],
  envGroupIds: ["evg-shared"],
  protectedStatus: "unprotected",
  networkIsolationEnabled: false,
  ipAllowList: [],
};

beforeEach(() => {
  setServices.mockReset();
  setServices.mockResolvedValue(true);
  setDatabases.mockReset();
  setDatabases.mockResolvedValue(true);
  setKeyValues.mockReset();
  setKeyValues.mockResolvedValue(true);
  setEnvGroups.mockReset();
  setEnvGroups.mockResolvedValue(true);
});

function renderDialog(onOpenChange = vi.fn()) {
  return render(
    <ManageResourcesDialog
      environment={env}
      services={[svc("api"), svc("web"), svc("worker")]}
      databases={[db("primary-db"), db("replica-db")]}
      keyValues={[kv("cache")]}
      open
      onOpenChange={onOpenChange}
    />,
  );
}

describe("ManageResourcesDialog", () => {
  it("pre-checks the environment's current services on the Services tab", () => {
    renderDialog();

    expect(screen.getByRole("checkbox", { name: /api/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /web/ })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: /worker/ })).not.toBeChecked();
  });

  it("full-replaces service membership: unchecking removes and checking assigns", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    renderDialog(onOpenChange);

    await user.click(screen.getByRole("checkbox", { name: /api/ }));
    await user.click(screen.getByRole("checkbox", { name: /web/ }));
    await user.click(screen.getByRole("checkbox", { name: /worker/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(setServices).toHaveBeenCalledTimes(1);
    const [id, name, ids] = setServices.mock.calls[0];
    expect(id).toBe("env-1");
    expect(name).toBe("staging");
    expect([...ids].sort()).toEqual(["web", "worker"]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("switches to the Databases tab and full-replaces database membership", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    renderDialog(onOpenChange);

    await user.click(screen.getByRole("tab", { name: "Databases" }));
    expect(screen.getByRole("checkbox", { name: /primary-db/ })).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: /replica-db/ }),
    ).not.toBeChecked();

    await user.click(screen.getByRole("checkbox", { name: /replica-db/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(setDatabases).toHaveBeenCalledTimes(1);
    const [id, name, ids] = setDatabases.mock.calls[0];
    expect(id).toBe("env-1");
    expect(name).toBe("staging");
    expect([...ids].sort()).toEqual(["primary-db", "replica-db"]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("switches to the Key Value tab and full-replaces key-value membership", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    renderDialog(onOpenChange);

    await user.click(screen.getByRole("tab", { name: "Key Value" }));
    expect(screen.getByRole("checkbox", { name: /cache/ })).toBeChecked();

    await user.click(screen.getByRole("checkbox", { name: /cache/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(setKeyValues).toHaveBeenCalledTimes(1);
    const [id, name, ids] = setKeyValues.mock.calls[0];
    expect(id).toBe("env-1");
    expect(name).toBe("staging");
    expect(ids).toEqual([]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("shows an empty state when the workspace has no services", () => {
    render(
      <ManageResourcesDialog
        environment={env}
        services={[]}
        databases={[db("primary-db")]}
        keyValues={[kv("cache")]}
        open
        onOpenChange={vi.fn()}
      />,
    );

    expect(
      screen.getByText("This workspace has no services to assign yet."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("full-replaces environment-group membership from workspace-scoped candidates", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    renderDialog(onOpenChange);

    await user.click(screen.getByRole("tab", { name: "Env Groups" }));
    expect(screen.getByRole("checkbox", { name: /shared/ })).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: /production-secrets/ }),
    ).not.toBeChecked();

    await user.click(screen.getByRole("checkbox", { name: /shared/ }));
    await user.click(
      screen.getByRole("checkbox", { name: /production-secrets/ }),
    );
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(setEnvGroups).toHaveBeenCalledWith("env-1", "staging", [
      "evg-production",
    ]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
