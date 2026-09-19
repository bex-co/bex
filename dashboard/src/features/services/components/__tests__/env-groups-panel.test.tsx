import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { EnvGroupView } from "@/features/env-groups/types";

// Control the data/behavior the panel sees by mocking the feature hooks; keep the
// real classifyEnvGroupError so the error-state routing is exercised for real.
const mockUseEnvGroups = vi.fn();
const mockCreateGroup = vi.fn();
const mockDeleteGroup = vi.fn();
const mockLinkGroup = vi.fn();
const mockUnlinkGroup = vi.fn();
// The service's own env-var keys, used to mark linked-group keys the service
// overrides (w6/067). Empty by default = no overrides.
const mockUseEnvVarKeys = vi.fn();
// The service's own secret-file names, used to mark a linked group's file of the
// same name (w2/m94/t002). Empty by default = no overrides.
const mockUseSecretFileNames = vi.fn();
// Link order (precedence) as the service read reports it; undefined = an older
// API that does not report it, which must claim no precedence at all.
let mockLinkedEnvGroupIds: string[] | undefined;

vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: (id: string) => ({
    service: { id, name: id, linkedEnvGroupIds: mockLinkedEnvGroupIds },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));

// The scope index the panel reads to learn which Environment the service lives
// in, so a create from an in-Environment service opens the dialog in that
// scope (w4/m111 t002). Mutable so a test can place the service in an env.
const scopeState = {
  projects: [],
  environments: [] as Array<{ id: string; name: string }>,
  byId: new Map<string, { id: string; name: string }>(),
  serviceEnvironmentById: new Map<string, string>(),
  loading: false,
  error: undefined as Error | undefined,
};

vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => ({
  useEnvGroupScopeIndex: () => scopeState,
  useWorkspaceEnvironmentIndex: () => scopeState,
}));

vi.mock("@/features/services/hooks/use-env-vars", () => ({
  useEnvVarKeys: (...a: unknown[]) => mockUseEnvVarKeys(...a),
}));

vi.mock("@/features/services/hooks/use-secret-files", () => ({
  useSecretFileNames: (...a: unknown[]) => mockUseSecretFileNames(...a),
}));

vi.mock(
  "@/features/env-groups/hooks/use-env-groups",
  async (importOriginal) => {
    const actual =
      await importOriginal<
        typeof import("@/features/env-groups/hooks/use-env-groups")
      >();
    return {
      ...actual,
      useEnvGroups: (...a: unknown[]) => mockUseEnvGroups(...a),
      useEnvGroupMutations: () => ({
        createGroup: mockCreateGroup,
        deleteGroup: mockDeleteGroup,
        linkGroup: mockLinkGroup,
        unlinkGroup: mockUnlinkGroup,
        busy: false,
      }),
    };
  },
);

import { EnvGroupsPanel } from "@/features/services/components/env-groups-panel";

function groupsResult(
  groups: EnvGroupView[],
  over: Partial<{ loading: boolean; error: Error | undefined }> = {},
) {
  return {
    groups,
    loading: false,
    error: undefined,
    refetch: vi.fn().mockResolvedValue(groups),
    ...over,
  };
}

beforeEach(() => {
  mockUseEnvGroups.mockReset();
  mockCreateGroup.mockReset().mockResolvedValue(true);
  mockDeleteGroup.mockReset().mockResolvedValue(true);
  mockLinkGroup.mockReset().mockResolvedValue(true);
  mockUnlinkGroup.mockReset().mockResolvedValue(true);
  mockUseEnvVarKeys.mockReset().mockReturnValue({
    keys: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  });
  mockUseSecretFileNames.mockReset().mockReturnValue({
    names: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  });
  mockLinkedEnvGroupIds = undefined;
});

describe("EnvGroupsPanel", () => {
  it("renders the empty state when there are no groups", () => {
    mockUseEnvGroups.mockReturnValue(groupsResult([]));
    render(<EnvGroupsPanel serviceId="web" />);
    expect(
      screen.getByText(/No workspace environment groups yet/),
    ).toBeInTheDocument();
  });

  it("lists groups with their var keys + file names and a Link action when not linked", () => {
    mockUseEnvGroups.mockReturnValue(
      groupsResult([
        {
          id: "eg1",
          name: "shared",
          ownerId: "tea-1",
          environmentId: null,
          createdAt: null,
          updatedAt: null,
          revision: null,
          availability: null,
          serviceLinks: ["other"],
          envVarKeys: ["FOO", "BAR"],
          secretFileNames: ["cert.pem"],
        },
      ]),
    );
    render(<EnvGroupsPanel serviceId="web" />);

    expect(screen.getByText("shared")).toBeInTheDocument();
    expect(screen.getByText("FOO")).toBeInTheDocument();
    expect(screen.getByText("BAR")).toBeInTheDocument();
    expect(screen.getByText("cert.pem")).toBeInTheDocument();
    // not linked to "web" => shows Link, not the Linked badge
    expect(screen.getByRole("button", { name: /^Link$/ })).toBeInTheDocument();
    expect(screen.queryByText("Linked")).not.toBeInTheDocument();
  });

  it("shows the Linked badge + Unlink action when the current service is a member", () => {
    mockUseEnvGroups.mockReturnValue(
      groupsResult([
        {
          id: "eg1",
          name: "shared",
          ownerId: "tea-1",
          environmentId: null,
          createdAt: null,
          updatedAt: null,
          revision: null,
          availability: null,
          serviceLinks: ["web"],
          envVarKeys: [],
          secretFileNames: [],
        },
      ]),
    );
    render(<EnvGroupsPanel serviceId="web" />);
    expect(screen.getByText("Linked")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Unlink/ })).toBeInTheDocument();
  });

  it("links the current service to a group", async () => {
    mockUseEnvGroups.mockReturnValue(
      groupsResult([
        {
          id: "eg1",
          name: "shared",
          ownerId: "tea-1",
          environmentId: null,
          createdAt: null,
          updatedAt: null,
          revision: null,
          availability: null,
          serviceLinks: [],
          envVarKeys: [],
          secretFileNames: [],
        },
      ]),
    );
    const user = userEvent.setup();
    render(<EnvGroupsPanel serviceId="web" />);

    await user.click(screen.getByRole("button", { name: /^Link$/ }));
    await waitFor(() =>
      expect(mockLinkGroup).toHaveBeenCalledWith("eg1", "web"),
    );
  });

  it("creates a group after validating the name", async () => {
    mockUseEnvGroups.mockReturnValue(groupsResult([]));
    const user = userEvent.setup();
    render(<EnvGroupsPanel serviceId="web" />);

    await user.click(screen.getByRole("button", { name: /Create group/ }));
    const nameInput = screen.getByLabelText("Group name");

    // blank name => no mutation
    await user.type(nameInput, "   ");
    expect(
      screen.getByRole("button", { name: "Create Environment Group" }),
    ).toBeDisabled();
    expect(mockCreateGroup).not.toHaveBeenCalled();

    await user.clear(nameInput);
    await user.type(nameInput, "shared");
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );
    expect(mockCreateGroup).toHaveBeenCalledWith({
      name: "shared",
      envVars: [],
      secretFiles: [],
      serviceIds: ["web"],
      // A workspace-scoped service creates a workspace-scoped group, exactly
      // as before w4/m111 — the scope is now explicit on the wire.
      environmentId: null,
    });
  });

  // w4/m111 t002: from an in-Environment service, Create group used to
  // pre-check the ONE link bex-api would refuse — the dialog defaulted to
  // workspace scope while the only offered service lived in an Environment,
  // so the create failed with ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH no matter
  // what the user clicked. The dialog now opens in the service's own scope.
  it("creates in the service's own Environment, keeping the pre-checked link", async () => {
    scopeState.environments = [{ id: "evm-qa", name: "qa-env" }];
    scopeState.serviceEnvironmentById = new Map([["web", "evm-qa"]]);
    try {
      mockUseEnvGroups.mockReturnValue(groupsResult([]));
      mockCreateGroup.mockResolvedValue("evg-1");
      const user = userEvent.setup();
      render(<EnvGroupsPanel serviceId="web" />);

      await user.click(screen.getByRole("button", { name: /Create group/ }));
      await user.type(
        screen.getByLabelText("Group name"),
        "qa-20260917-p3-evg2",
      );
      // The service is still offered and still checked — it is compatible now.
      expect(screen.getByRole("checkbox", { name: /web/ })).toBeChecked();
      await user.click(
        screen.getByRole("button", { name: "Create Environment Group" }),
      );

      expect(mockCreateGroup).toHaveBeenCalledWith({
        name: "qa-20260917-p3-evg2",
        envVars: [],
        secretFiles: [],
        serviceIds: ["web"],
        environmentId: "evm-qa",
      });
    } finally {
      scopeState.environments = [];
      scopeState.serviceEnvironmentById = new Map();
    }
  });

  it("marks a linked group's keys that the service's own variables override (w6/067)", () => {
    mockUseEnvVarKeys.mockReturnValue({
      keys: [{ id: "SHARED_KEY", key: "SHARED_KEY" }],
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    mockUseEnvGroups.mockReturnValue(
      groupsResult([
        {
          id: "eg1",
          name: "shared",
          ownerId: "tea-1",
          environmentId: null,
          createdAt: null,
          updatedAt: null,
          revision: null,
          availability: null,
          serviceLinks: ["web"],
          envVarKeys: ["GROUP_ONLY", "SHARED_KEY"],
          secretFileNames: [],
        },
      ]),
    );
    render(<EnvGroupsPanel serviceId="web" />);

    // The colliding key is struck through with a title explaining why; the
    // group-only key stays unmarked.
    const shared = screen.getByText("SHARED_KEY");
    expect(shared.tagName).toBe("S");
    expect(shared.closest("[title]")).toHaveAttribute(
      "title",
      "Overridden by this service's own SHARED_KEY environment variable",
    );
    expect(screen.getByText("GROUP_ONLY").tagName).not.toBe("S");
    expect(
      screen.getByText(
        "Struck-through keys are overridden by this service's own environment variables.",
      ),
    ).toBeInTheDocument();
  });

  it("does not mark keys on a group that is not linked to this service (w6/067)", () => {
    mockUseEnvVarKeys.mockReturnValue({
      keys: [{ id: "SHARED_KEY", key: "SHARED_KEY" }],
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    mockUseEnvGroups.mockReturnValue(
      groupsResult([
        {
          id: "eg1",
          name: "shared",
          ownerId: "tea-1",
          environmentId: null,
          createdAt: null,
          updatedAt: null,
          revision: null,
          availability: null,
          serviceLinks: ["other"],
          envVarKeys: ["SHARED_KEY"],
          secretFileNames: [],
        },
      ]),
    );
    render(<EnvGroupsPanel serviceId="web" />);

    // An unlinked group feeds nothing into this service, so nothing shadows.
    expect(screen.getByText("SHARED_KEY").tagName).not.toBe("S");
    expect(
      screen.queryByText(/overridden by this service/i),
    ).not.toBeInTheDocument();
  });

  it("does not expose workspace-destructive group deletion", () => {
    mockUseEnvGroups.mockReturnValue(
      groupsResult([
        {
          id: "eg1",
          name: "shared",
          ownerId: "tea-1",
          environmentId: null,
          createdAt: null,
          updatedAt: null,
          revision: null,
          availability: null,
          serviceLinks: [],
          envVarKeys: [],
          secretFileNames: [],
        },
      ]),
    );
    render(<EnvGroupsPanel serviceId="web" />);
    expect(
      screen.queryByRole("button", { name: "Delete" }),
    ).not.toBeInTheDocument();
    expect(mockDeleteGroup).not.toHaveBeenCalled();
  });
});

// Group-vs-group precedence (w2/m94/t003, from w1/091). Linking B then A means
// A wins: the operator emits each linked group's Secret as an envFrom source in
// spec.envFromSecrets order and Kubernetes lets the LAST source win, so the
// most recently linked group is what the service actually runs. At filing time
// the page listed A above B by accident, not by rule, and marked neither key.
describe("EnvGroupsPanel group-vs-group precedence", () => {
  function twoLinkedGroups() {
    return groupsResult([
      {
        id: "egB",
        name: "beta",
        ownerId: "tea-1",
        environmentId: null,
        createdAt: null,
        updatedAt: null,
        revision: null,
        availability: null,
        serviceLinks: ["web"],
        envVarKeys: ["MESSAGE"],
        secretFileNames: ["qa.txt"],
      },
      {
        id: "egA",
        name: "alpha",
        ownerId: "tea-1",
        environmentId: null,
        createdAt: null,
        updatedAt: null,
        revision: null,
        availability: null,
        serviceLinks: ["web"],
        envVarKeys: ["MESSAGE"],
        secretFileNames: ["qa.txt"],
      },
    ]);
  }

  it("orders linked groups by precedence and marks the loser's key and file", () => {
    // B linked first, then A => A wins and is shown first.
    mockLinkedEnvGroupIds = ["egB", "egA"];
    mockUseEnvGroups.mockReturnValue(twoLinkedGroups());
    render(<EnvGroupsPanel serviceId="web" />);

    const names = screen
      .getAllByRole("link")
      .map((node) => node.textContent?.trim());
    expect(names.indexOf("alpha")).toBeLessThan(names.indexOf("beta"));

    // Exactly one MESSAGE badge is struck through — beta's — and its tooltip
    // names alpha as the winner.
    const struck = document.querySelectorAll("s");
    const strikeText = Array.from(struck).map((node) => node.textContent);
    expect(strikeText).toContain("MESSAGE");
    expect(strikeText).toContain("qa.txt");
    expect(
      document.querySelectorAll('[title="Overridden by alpha, which is linked later"]'),
    ).toHaveLength(2);
    // The winner is not marked.
    expect(strikeText).toHaveLength(2);
  });

  it("claims no precedence when the API does not report link order", () => {
    mockLinkedEnvGroupIds = undefined;
    mockUseEnvGroups.mockReturnValue(twoLinkedGroups());
    render(<EnvGroupsPanel serviceId="web" />);
    // Marking one at random would be worse than marking neither: before m94 the
    // page implied a winner it had not computed.
    expect(document.querySelectorAll("s")).toHaveLength(0);
  });

  it("lets the service's own secret file beat every linked group's", () => {
    mockLinkedEnvGroupIds = ["egB", "egA"];
    mockUseEnvGroups.mockReturnValue(twoLinkedGroups());
    mockUseSecretFileNames.mockReturnValue({
      names: [{ id: "qa.txt", name: "qa.txt" }],
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    render(<EnvGroupsPanel serviceId="web" />);

    // Both groups' qa.txt lose to the service's own file, so the service-wins
    // tooltip replaces the group-vs-group one on the winning group too.
    expect(
      document.querySelectorAll(
        '[title="Overridden by this service\'s own qa.txt secret file"]',
      ),
    ).toHaveLength(2);
  });
});
