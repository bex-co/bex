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

vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: (id: string) => ({
    service: { id, name: id },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
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
    });
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
