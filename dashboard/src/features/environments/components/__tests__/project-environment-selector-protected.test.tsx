import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectEnvironmentSelector } from "../project-environment-selector";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }
});

vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [
      {
        id: "prj-shop",
        name: "Shop",
        ownerId: "tea-1",
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
      },
    ],
    loading: false,
  }),
}));

vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({
    environments: [
      {
        id: "evm-prod",
        projectId: "prj-shop",
        name: "production",
        protectedStatus: "protected",
      },
      {
        id: "evm-staging",
        projectId: "prj-shop",
        name: "staging",
        protectedStatus: "unprotected",
      },
    ],
    loading: false,
  }),
}));

const REASON =
  "Only an admin can create resources in a protected environment or move them into or out of it.";

function renderSelector(onEnvironmentChange = vi.fn()) {
  render(
    <ProjectEnvironmentSelector
      projectId="prj-shop"
      environmentId={null}
      onProjectChange={vi.fn()}
      onEnvironmentChange={onEnvironmentChange}
    />,
  );
  return onEnvironmentChange;
}

// w3/m281: Render lets only Admins create resources in a protected
// environment, so a non-Admin's create form offers it disabled, with the
// reason as the picker's description.
describe("ProjectEnvironmentSelector protected placement", () => {
  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  });

  it("disables a protected environment for a non-admin and says why", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    const onEnvironmentChange = renderSelector();
    const picker = screen.getByRole("combobox", { name: "Environment" });
    expect(picker).toHaveAccessibleDescription(REASON);

    const user = userEvent.setup();
    await user.click(picker);
    const protectedOption = await screen.findByRole("option", {
      name: "production (admins only)",
    });
    expect(protectedOption).toHaveAttribute("aria-disabled", "true");
    await user.click(protectedOption);
    expect(onEnvironmentChange).not.toHaveBeenCalledWith("evm-prod");
    await user.click(screen.getByRole("option", { name: "staging" }));
    expect(onEnvironmentChange).toHaveBeenCalledWith("evm-staging");
  });

  it("offers a protected environment to an admin", async () => {
    const onEnvironmentChange = renderSelector();
    const picker = screen.getByRole("combobox", { name: "Environment" });
    expect(picker).not.toHaveAccessibleDescription(REASON);
    expect(screen.queryByText(REASON)).toBeNull();

    const user = userEvent.setup();
    await user.click(picker);
    await user.click(await screen.findByRole("option", { name: "production" }));
    expect(onEnvironmentChange).toHaveBeenCalledWith("evm-prod");
  });
});
