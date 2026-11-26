import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

const nav = vi.hoisted(() => ({ navigate: vi.fn() }));
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useNavigate: () => nav.navigate,
  Link: ({ to, children, ...rest }: { to: string; children: ReactNode }) => (
    <a href={to} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-a" }),
}));

const query = vi.hoisted(() => ({
  result: {} as Record<string, unknown>,
  variables: [] as unknown[],
}));
vi.mock("@apollo/client/react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@apollo/client/react")>()),
  useQuery: (_doc: unknown, options: { variables: unknown }) => {
    query.variables.push(options.variables);
    return query.result;
  },
}));

import {
  SandboxLinkRedirect,
  SnapshotLinkRedirect,
} from "@/features/sandbox-group-pages/components/resource-link-redirect";

const GROUP = "sbg-dc44tha9086ovmjtv6c0";

describe("links to a sandbox or a snapshot by id (w1/m658)", () => {
  beforeEach(() => {
    nav.navigate.mockReset();
    query.variables = [];
  });

  it("lands a sandbox on its group's Sandboxes page with its drawer open", () => {
    query.result = {
      data: {
        sandbox: {
          id: "sbx-0000000000000000000a",
          sandboxGroupId: GROUP,
          workspace: "tea-b",
        },
      },
      loading: false,
    };
    render(<SandboxLinkRedirect sandboxId="sbx-0000000000000000000a" />);
    // Read with no workspace named: bex-api finds it in any the caller belongs to.
    expect(query.variables[0]).toEqual({ id: "sbx-0000000000000000000a" });
    expect(nav.navigate).toHaveBeenCalledWith(
      expect.objectContaining({
        to: "/sandbox-groups/$sandboxGroupId",
        params: { sandboxGroupId: GROUP },
        search: { sandbox: "sbx-0000000000000000000a" },
        replace: true,
      }),
    );
  });

  it("lands a snapshot on its group's Snapshots page, read in the selected workspace", () => {
    query.result = {
      data: {
        sandboxSnapshot: {
          id: "snp-0000000000000000000a",
          sandboxGroupId: GROUP,
        },
      },
      loading: false,
    };
    render(<SnapshotLinkRedirect snapshotId="snp-0000000000000000000a" />);
    expect(query.variables[0]).toEqual({
      id: "snp-0000000000000000000a",
      ownerId: "tea-a",
    });
    expect(nav.navigate).toHaveBeenCalledWith(
      expect.objectContaining({
        to: "/sandbox-groups/$sandboxGroupId/snapshots",
        search: { snapshot: "snp-0000000000000000000a" },
      }),
    );
  });

  it("finds nothing for an unknown id or another workspace's", () => {
    query.result = { data: { sandbox: null }, loading: false };
    render(<SandboxLinkRedirect sandboxId="sbx-0000000000000000000z" />);
    expect(nav.navigate).not.toHaveBeenCalled();
    expect(screen.getByText("Sandbox not found")).toBeInTheDocument();
    expect(
      screen.getByText(/sbx-0000000000000000000z isn't a sandbox you can see/),
    ).toBeInTheDocument();
  });

  it("finds nothing for a snapshot outside the workspace", () => {
    query.result = { data: { sandboxSnapshot: null }, loading: false };
    render(<SnapshotLinkRedirect snapshotId="snp-0000000000000000000z" />);
    expect(nav.navigate).not.toHaveBeenCalled();
    expect(screen.getByText("Snapshot not found")).toBeInTheDocument();
  });
});
