import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { EnvGroupMetadata } from "@/features/env-groups/components/env-group-metadata";
import type { EnvGroupView } from "@/features/env-groups/types";

vi.mock("@/features/workspaces/context/hooks", async () => {
  const { mockWorkspaceContext } = await import("@/test/mocks/workspace");
  return mockWorkspaceContext("tea-known");
});

const group = (ownerId: string) =>
  ({
    id: "evg-1",
    name: "grp",
    ownerId,
    createdAt: null,
    updatedAt: null,
  }) as unknown as EnvGroupView;

// w4/514: the Owner row showed the raw `tea-…` id instead of the workspace name.
describe("EnvGroupMetadata owner", () => {
  it("names the owning workspace and keeps its id in the tooltip", () => {
    render(<EnvGroupMetadata group={group("tea-known")} />);
    const owner = screen.getByText("Test");
    expect(owner).toHaveAttribute("title", "tea-known");
    expect(screen.queryByText("tea-known")).not.toBeInTheDocument();
  });

  it("falls back to the id when the workspace is not in the viewer's list", () => {
    render(<EnvGroupMetadata group={group("tea-other")} />);
    expect(screen.getByText("tea-other")).toBeInTheDocument();
  });
});
