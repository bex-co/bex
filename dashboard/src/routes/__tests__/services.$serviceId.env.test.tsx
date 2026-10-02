import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import userEvent from "@testing-library/user-event";

vi.mock("@/features/services/components/service-environment-editor", () => ({
  ServiceEnvironmentEditor: () => <div>environment editor</div>,
}));
const createGroup = vi.fn();
const refetch = vi.fn().mockResolvedValue([]);
const scopeState = {
  ownerId: "tea-a",
  ready: true,
  loading: false,
  error: undefined as Error | undefined,
  retry: vi.fn(),
  environments: [] as Array<{ id: string; name: string }>,
  serviceEnvironmentById: new Map<string, string>(),
};
vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => ({
  useEnvGroupScopeIndex: () => scopeState,
}));
vi.mock(
  "@/features/env-groups/hooks/use-env-groups",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("@/features/env-groups/hooks/use-env-groups")
    >()),
    useEnvGroups: () => ({
      groups: [],
      loading: false,
      error: undefined,
      refetch,
    }),
    useEnvGroupMutations: () => ({ createGroup, busy: false }),
  }),
);
vi.mock("@/features/services/hooks/use-env-vars", () => ({
  useEnvVarKeys: () => ({ keys: [] }),
}));
vi.mock("@/features/services/hooks/use-secret-files", () => ({
  useSecretFileNames: () => ({ names: [] }),
}));
const useServer = vi.fn();
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: (...args: unknown[]) => useServer(...args),
}));

import { ServiceEnvPage } from "@/routes/services.$serviceId.env";

describe("ServiceEnvPage group creation", () => {
  beforeEach(() => {
    createGroup.mockReset().mockResolvedValue("eg-new");
    scopeState.loading = true;
    scopeState.ready = false;
    scopeState.error = undefined;
    scopeState.environments = [];
    scopeState.serviceEnvironmentById = new Map();
    scopeState.retry.mockReset();
    refetch.mockClear();
    useServer.mockReturnValue({
      service: { id: "srv-1", name: "qa-service" },
      loading: false,
      refetch,
    });
  });

  it.each([
    { button: "toolbar", index: 0, resolveBeforeOpen: true },
    { button: "panel", index: 1, resolveBeforeOpen: true },
    { button: "toolbar", index: 0, resolveBeforeOpen: false },
    { button: "panel", index: 1, resolveBeforeOpen: false },
  ])(
    "first $button opening uses the resolved scope (resolve before open: $resolveBeforeOpen)",
    async ({ index, resolveBeforeOpen }) => {
      const user = userEvent.setup();
      const { rerender } = render(<ServiceEnvPage serviceId="srv-1" />);
      const resolveScope = () => {
        scopeState.loading = false;
        scopeState.ready = true;
        scopeState.environments = [{ id: "env-qa", name: "QA" }];
        scopeState.serviceEnvironmentById = new Map([["srv-1", "env-qa"]]);
        rerender(<ServiceEnvPage serviceId="srv-1" />);
      };
      if (resolveBeforeOpen) resolveScope();
      await user.click(
        screen.getAllByRole("button", { name: "Create group" })[index],
      );
      if (!resolveBeforeOpen) {
        expect(
          screen.getByRole("button", { name: "Create Environment Group" }),
        ).toBeDisabled();
        expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
        resolveScope();
      }
      expect(
        screen.getByRole("combobox", { name: "Environment" }),
      ).toHaveTextContent("QA");
      expect(
        screen.getByRole("checkbox", { name: /qa-service/ }),
      ).toBeChecked();
      await user.type(screen.getByLabelText("Group name"), "first-create");
      await user.click(
        screen.getByRole("button", { name: "Create Environment Group" }),
      );
      expect(createGroup).toHaveBeenCalledWith(
        expect.objectContaining({
          environmentId: "env-qa",
          serviceIds: ["srv-1"],
        }),
      );
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    },
  );

  it("keeps Workspace defaults on cancel and reopen", async () => {
    const user = userEvent.setup();
    scopeState.ready = true;
    scopeState.loading = false;
    render(<ServiceEnvPage serviceId="srv-1" />);
    await user.click(
      screen.getAllByRole("button", { name: "Create group" })[0],
    );
    await user.type(screen.getByLabelText("Group name"), "discarded");
    await user.click(screen.getByRole("checkbox", { name: /qa-service/ }));
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await user.click(
      screen.getAllByRole("button", { name: "Create group" })[1],
    );
    expect(screen.getByLabelText("Group name")).toHaveValue("");
    expect(
      screen.getByRole("combobox", { name: "Environment" }),
    ).toHaveTextContent("Workspace");
    expect(screen.getByRole("checkbox", { name: /qa-service/ })).toBeChecked();
    await user.type(screen.getByLabelText("Group name"), "workspace-create");
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );
    expect(createGroup).toHaveBeenCalledWith(
      expect.objectContaining({ environmentId: null, serviceIds: ["srv-1"] }),
    );
  });

  it("shows lookup errors and retries the scope and current service", async () => {
    const user = userEvent.setup();
    scopeState.loading = false;
    scopeState.error = new Error("forbidden");
    const { rerender } = render(<ServiceEnvPage serviceId="srv-1" />);
    await user.click(
      screen.getAllByRole("button", { name: "Create group" })[0],
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Couldn't load");
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(scopeState.retry).toHaveBeenCalledOnce();
    expect(refetch).toHaveBeenCalledOnce();
    scopeState.error = undefined;
    scopeState.ready = true;
    scopeState.environments = [{ id: "env-qa", name: "QA" }];
    scopeState.serviceEnvironmentById = new Map([["srv-1", "env-qa"]]);
    rerender(<ServiceEnvPage serviceId="srv-1" />);
    expect(screen.getByRole("checkbox", { name: /qa-service/ })).toBeChecked();
    expect(
      screen.getByRole("combobox", { name: "Environment" }),
    ).toHaveTextContent("QA");
  });
});

const NOTICE =
  "Saved changes aren't live yet. Use a standard deploy to apply them.";

// This page lists saved values. Save only, cancellation, rollback and a restart
// of that release can leave them different from the running configuration.
describe("ServiceEnvPage undeployed-changes notice", () => {
  beforeEach(() => useServer.mockReset());

  it("explains how to apply saved values that are not running", () => {
    useServer.mockReturnValue({ service: { undeployedChanges: true } });
    render(<ServiceEnvPage serviceId="srv-1" />);
    expect(screen.getByText(NOTICE)).toBeInTheDocument();
    expect(screen.getByText("environment editor")).toBeInTheDocument();
  });

  it("says nothing on an ordinary service, or before the service loads", () => {
    useServer.mockReturnValue({ service: { undeployedChanges: false } });
    const { unmount } = render(<ServiceEnvPage serviceId="srv-1" />);
    expect(screen.queryByText(NOTICE)).not.toBeInTheDocument();
    unmount();

    useServer.mockReturnValue({ service: null });
    render(<ServiceEnvPage serviceId="srv-1" />);
    expect(screen.queryByText(NOTICE)).not.toBeInTheDocument();
  });

  it("reads the layout's cached document without starting its own poll", () => {
    useServer.mockReturnValue({ service: null });
    render(<ServiceEnvPage serviceId="srv-1" />);
    // A second polling consumer drifts into separate round trips (use-server.ts);
    // the detail layout owns the cadence.
    expect(useServer).toHaveBeenCalledWith("srv-1", { poll: false });
  });

  it("follows the authoritative flag as Save only settles and a revert clears it", () => {
    useServer.mockReturnValue({ service: { undeployedChanges: false } });
    const { rerender } = render(<ServiceEnvPage serviceId="srv-1" />);
    expect(screen.queryByText(NOTICE)).not.toBeInTheDocument();

    useServer.mockReturnValue({ service: { undeployedChanges: true } });
    rerender(<ServiceEnvPage serviceId="srv-1" />);
    expect(screen.getByText(NOTICE)).toBeInTheDocument();

    useServer.mockReturnValue({ service: { undeployedChanges: false } });
    rerender(<ServiceEnvPage serviceId="srv-1" />);
    expect(screen.queryByText(NOTICE)).not.toBeInTheDocument();
  });
});
