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
  ipAllowListEntries: [],
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

  it.each([
    { tab: "Databases", member: "primary-db", mutate: setDatabases },
    { tab: "Key Value", member: "cache", mutate: setKeyValues },
  ])(
    "keeps $tab selections open after a refused replacement",
    async ({ tab, member, mutate }) => {
      mutate.mockResolvedValue(false);
      const onOpenChange = vi.fn();
      const user = userEvent.setup();
      renderDialog(onOpenChange);

      await user.click(screen.getByRole("tab", { name: tab }));
      await user.click(
        screen.getByRole("checkbox", { name: new RegExp(member) }),
      );
      await user.click(screen.getByRole("button", { name: "Save" }));

      expect(mutate).toHaveBeenCalledWith("env-1", "staging", []);
      expect(onOpenChange).not.toHaveBeenCalled();
      expect(screen.getByRole("dialog")).toBeInTheDocument();
      expect(
        screen.getByRole("checkbox", { name: new RegExp(member) }),
      ).not.toBeChecked();

      mutate.mockResolvedValue(true);
      await user.click(screen.getByRole("button", { name: "Save" }));
      expect(mutate).toHaveBeenLastCalledWith("env-1", "staging", []);
      expect(onOpenChange).toHaveBeenCalledWith(false);
    },
  );

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

// w4/133: Radix unmounts inactive TabsContent, and the checklist used to seed
// its selection on mount — so looking at another resource kind destroyed the
// draft and reseeded from unchanged server membership. Live, an unchecked
// service silently checked itself again after a Services → Databases → Services
// round trip, with no Save in between and no membership mutation on the server.
describe("ManageResourcesDialog draft retention across tabs", () => {
  const tabs = [
    { name: "Databases", label: "Databases" },
    { name: "Key Value", label: "Key Value" },
    { name: "Env Groups", label: "Env Groups" },
  ];

  for (const tab of tabs) {
    it(`keeps an unsaved Services edit across a round trip through ${tab.label}`, async () => {
      const user = userEvent.setup();
      renderDialog();

      await user.click(screen.getByRole("checkbox", { name: /api/ }));
      expect(screen.getByRole("checkbox", { name: /api/ })).not.toBeChecked();

      await user.click(screen.getByRole("tab", { name: tab.name }));
      await user.click(screen.getByRole("tab", { name: "Services" }));

      expect(screen.getByRole("checkbox", { name: /api/ })).not.toBeChecked();
    });
  }

  it("saves the retained draft, not the persisted membership, after a round trip", async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.click(screen.getByRole("checkbox", { name: /api/ }));
    await user.click(screen.getByRole("tab", { name: "Databases" }));
    await user.click(screen.getByRole("tab", { name: "Services" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    // The last member was unchecked, so the complete replacement set is empty —
    // and it must still be submitted as [], not withheld.
    expect(setServices).toHaveBeenCalledTimes(1);
    expect(setServices.mock.calls[0][2]).toEqual([]);
    expect(setDatabases).not.toHaveBeenCalled();
    expect(setKeyValues).not.toHaveBeenCalled();
    expect(setEnvGroups).not.toHaveBeenCalled();
  });

  it("keeps the four drafts independent, and saves only the active tab's", async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.click(screen.getByRole("checkbox", { name: /web/ }));

    await user.click(screen.getByRole("tab", { name: "Databases" }));
    await user.click(screen.getByRole("checkbox", { name: /replica-db/ }));

    await user.click(screen.getByRole("tab", { name: "Key Value" }));
    await user.click(screen.getByRole("checkbox", { name: /cache/ }));

    await user.click(screen.getByRole("tab", { name: "Env Groups" }));
    await user.click(
      screen.getByRole("checkbox", { name: /production-secrets/ }),
    );

    // Every edit survived three further tab changes.
    await user.click(screen.getByRole("tab", { name: "Services" }));
    expect(screen.getByRole("checkbox", { name: /web/ })).toBeChecked();
    await user.click(screen.getByRole("tab", { name: "Databases" }));
    expect(screen.getByRole("checkbox", { name: /replica-db/ })).toBeChecked();
    await user.click(screen.getByRole("tab", { name: "Key Value" }));
    expect(screen.getByRole("checkbox", { name: /cache/ })).not.toBeChecked();
    await user.click(screen.getByRole("tab", { name: "Env Groups" }));
    expect(
      screen.getByRole("checkbox", { name: /production-secrets/ }),
    ).toBeChecked();

    // Saving writes ONE tab's replacement set — the active one — and the three
    // other drafts are discarded with the dialog rather than silently written.
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(setEnvGroups).toHaveBeenCalledTimes(1);
    expect([...setEnvGroups.mock.calls[0][2]].sort()).toEqual([
      "evg-production",
      "evg-shared",
    ]);
    expect(setServices).not.toHaveBeenCalled();
    expect(setDatabases).not.toHaveBeenCalled();
    expect(setKeyValues).not.toHaveBeenCalled();
  });

  it("keeps only the active tab's controls in the accessibility tree", async () => {
    const user = userEvent.setup();
    renderDialog();

    expect(screen.getByRole("checkbox", { name: /api/ })).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: /replica-db/ }),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Databases" }));
    expect(
      screen.getByRole("checkbox", { name: /replica-db/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: /worker/ }),
    ).not.toBeInTheDocument();
  });

  it("a failed Save keeps the dialog open with the draft intact", async () => {
    const onOpenChange = vi.fn();
    setServices.mockResolvedValue(false);
    const user = userEvent.setup();
    renderDialog(onOpenChange);

    await user.click(screen.getByRole("checkbox", { name: /web/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.getByRole("checkbox", { name: /web/ })).toBeChecked();
  });

  it("an unchanged refetch of the same membership does not stomp a draft", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const { rerender } = renderDialog(onOpenChange);

    await user.click(screen.getByRole("checkbox", { name: /api/ }));
    expect(screen.getByRole("checkbox", { name: /api/ })).not.toBeChecked();

    // A poll tick hands down an equal-but-new array instance, the shape that
    // would resurrect the draft through any sync effect keyed on identity.
    rerender(
      <ManageResourcesDialog
        environment={{ ...env, serviceIds: ["api"] }}
        services={[svc("api"), svc("web"), svc("worker")]}
        databases={[db("primary-db"), db("replica-db")]}
        keyValues={[kv("cache")]}
        open
        onOpenChange={onOpenChange}
      />,
    );

    expect(screen.getByRole("checkbox", { name: /api/ })).not.toBeChecked();
  });

  it("discards drafts on close and reseeds from persisted membership on reopen", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const { rerender } = renderDialog(onOpenChange);

    await user.click(screen.getByRole("checkbox", { name: /api/ }));
    expect(screen.getByRole("checkbox", { name: /api/ })).not.toBeChecked();

    // Cancel: the dialog's own close path, which unmounts the form.
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);

    const props = {
      environment: env,
      services: [svc("api"), svc("web"), svc("worker")],
      databases: [db("primary-db"), db("replica-db")],
      keyValues: [kv("cache")],
      onOpenChange,
    };
    rerender(<ManageResourcesDialog {...props} open={false} />);
    rerender(<ManageResourcesDialog {...props} open />);

    // Draft gone, persisted membership back — the behaviour the per-mount seed
    // was originally right about, and which lifting the drafts must preserve.
    expect(screen.getByRole("checkbox", { name: /api/ })).toBeChecked();
    expect(setServices).not.toHaveBeenCalled();
  });
});
