import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ScopeSelect } from "@/features/env-groups/components/scope-select";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { ProjectView } from "@/features/projects/hooks/use-projects";
import { environmentNameIn } from "@/features/env-groups/lib/scope";

// w4/215: two projects each with an environment named "shared" showed two
// identical choices, and the saved scope read the bare name.
const environment = (id: string, projectId: string) =>
  ({
    id,
    projectId,
    name: "shared",
    serviceIds: [],
    databaseIds: [],
    keyValueIds: [],
  }) as unknown as EnvironmentView;
const project = (id: string, name: string) =>
  ({
    id,
    name,
    ownerId: "tea-1",
    serviceIds: [],
    databaseIds: [],
    keyValueIds: [],
  }) as ProjectView;
const environments = [
  environment("evm-a", "prj-a"),
  environment("evm-b", "prj-b"),
];
const projects = [project("prj-a", "Alpha"), project("prj-b", "Beta")];

describe("ScopeSelect (w4/215)", () => {
  it("names each same-named environment's project and submits its own id", async () => {
    const onValueChange = vi.fn();
    const user = userEvent.setup();
    render(
      <ScopeSelect
        id="scope"
        value="__workspace__"
        environments={environments}
        projects={projects}
        onValueChange={onValueChange}
      />,
    );
    await user.click(screen.getByRole("combobox", { name: "Environment" }));
    expect(
      screen.getByRole("option", { name: "Alpha / shared" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("option", { name: "Beta / shared" }));
    expect(onValueChange).toHaveBeenCalledWith("evm-b");
  });

  it("names a saved scope's project too, or nothing when the index lacks it", () => {
    const index = {
      byId: new Map(environments.map((item) => [item.id, item])),
      projects,
    };
    expect(environmentNameIn(index, "evm-a")).toBe("Alpha / shared");
    expect(environmentNameIn(index, "evm-missing")).toBeUndefined();
  });
});
