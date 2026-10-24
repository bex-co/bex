import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const mocks = vi.hoisted(() => ({
  saveACL: vi.fn(async () => true),
  plan: "hobby" as string,
}));

vi.mock("@/features/environments/hooks/use-set-environment-acl", () => ({
  useSetEnvironmentACL: () => ({ saveACL: mocks.saveACL, saving: false }),
}));
vi.mock("@/features/workspaces/context", () => ({
  useWorkspace: () => ({ currentWorkspace: { plan: mocks.plan } }),
}));

import { EnvironmentSettingsDialog } from "@/features/environments/components/environment-settings-dialog";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";

function environment(
  entries: Array<{ cidrBlock: string; description: string }>,
): EnvironmentView {
  return {
    id: "evm-1",
    projectId: "prj-1",
    name: "production",
    ownerId: "tea-a",
    createdAt: null,
    serviceIds: [],
    databaseIds: [],
    keyValueIds: [],
    envGroupIds: [],
    protectedStatus: "unprotected",
    networkIsolationEnabled: false,
    ipAllowListEntries: entries,
  };
}

beforeEach(() => {
  mocks.saveACL.mockClear();
  mocks.plan = "hobby";
});

// w8/m137: below the Scale plan an environment's inbound rules cannot be added
// or edited; a legacy restriction can be reset to allow-all.
describe("EnvironmentSettingsDialog inbound IP rules by plan", () => {
  it("hides the editor on Hobby and resets a legacy restriction to allow-all", async () => {
    const user = userEvent.setup();
    render(
      <EnvironmentSettingsDialog
        environment={environment([
          { cidrBlock: "203.0.113.0/24", description: "office" },
        ])}
        open
        onOpenChange={vi.fn()}
      />,
    );
    expect(screen.getByText(/require the Scale plan/)).toBeInTheDocument();
    expect(screen.queryByLabelText("New source CIDR block")).toBeNull();
    expect(screen.queryByRole("button", { name: "Add source" })).toBeNull();

    await user.click(
      screen.getByRole("button", { name: "Reset to allow all" }),
    );
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(mocks.saveACL).toHaveBeenCalledWith(
      "evm-1",
      "production",
      expect.objectContaining({
        ipAllowListEntries: [
          { cidrBlock: "0.0.0.0/0", description: "Allow all (default)" },
          { cidrBlock: "::/0", description: "Allow all (default)" },
        ],
      }),
    );
  });

  it("keeps the editor on Scale", () => {
    mocks.plan = "scale";
    render(
      <EnvironmentSettingsDialog
        environment={environment([])}
        open
        onOpenChange={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Add source" }),
    ).toBeInTheDocument();
  });
});
