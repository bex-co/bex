import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectEnvironmentSelector } from "../project-environment-selector";

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

const projectsState = {
  projects: [
    {
      id: "prj-empty",
      name: "Empty Project",
      ownerId: "tea-1",
      serviceIds: [],
      databaseIds: [],
      keyValueIds: [],
    },
  ],
};
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({ ...projectsState, loading: false }),
}));

const environmentsState: {
  environments: Array<{ id: string; projectId: string; name: string }>;
} = { environments: [] };
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({ ...environmentsState, loading: false }),
}));

beforeEach(() => {
  environmentsState.environments = [];
});

// w6/m48 t003 rewrote environments.assignmentHint so Project-only selection no
// longer reads as joining the Project; w6/042 then found the replacement copy's
// own claim ("pick or create an Environment") was also false — the selector has
// no inline create affordance. These pin both: honest copy, honest option list.
describe("ProjectEnvironmentSelector", () => {
  function renderSelector(projectId: string | null = "prj-empty") {
    return render(
      <ProjectEnvironmentSelector
        projectId={projectId}
        environmentId={null}
        onProjectChange={vi.fn()}
        onEnvironmentChange={vi.fn()}
      />,
    );
  }

  it("does not promise an inline create action and points at the Project page instead (w6/042)", () => {
    renderSelector();

    // The old copy claimed the user could "pick or create an Environment" right
    // here; the control only ever offers "No environment" + existing ones.
    expect(screen.queryByText(/pick or create/i)).not.toBeInTheDocument();
    expect(
      screen.getByText(/create one from the Project's page first/i),
    ).toBeInTheDocument();
  });

  it("offers no phantom create affordance in the Environment option list (w6/042)", async () => {
    const user = userEvent.setup();
    renderSelector();

    await user.click(screen.getByRole("combobox", { name: "Environment" }));

    const options = screen.getAllByRole("option");
    expect(options.map((option) => option.textContent)).toEqual([
      "No environment",
    ]);
    expect(
      screen.queryByRole("option", { name: /create/i }),
    ).not.toBeInTheDocument();
  });

  it("lists existing Environments without adding a create item", async () => {
    environmentsState.environments = [
      { id: "env-prod", projectId: "prj-empty", name: "Production" },
    ];
    const user = userEvent.setup();
    renderSelector();

    await user.click(screen.getByRole("combobox", { name: "Environment" }));

    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["No environment", "Production"]);
  });
});
