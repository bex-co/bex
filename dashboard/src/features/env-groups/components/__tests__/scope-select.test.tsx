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

  // w4/270: a long "Project / Environment" label widened the Move/Clone
  // dialogs to ~806px at 390px and ran the trigger and menu off screen.
  it("wraps a long qualified scope inside the containing width (w4/270)", async () => {
    const user = userEvent.setup();
    const long = "qa-project-with-a-long-valid-name-雪";
    render(
      <ScopeSelect
        id="scope"
        value="evm-a"
        environments={environments}
        projects={[project("prj-a", long), project("prj-b", "Beta")]}
        onValueChange={vi.fn()}
      />,
    );
    const trigger = screen.getByRole("combobox", { name: "Environment" });
    expect(trigger).toHaveTextContent(`${long} / shared`);
    expect(trigger).toHaveClass(
      "max-w-full",
      "wrap-anywhere",
      "*:data-[slot=select-value]:line-clamp-none",
    );
    await user.click(trigger);
    const option = screen.getByRole("option", { name: `${long} / shared` });
    expect(option).toHaveClass("wrap-anywhere");
    expect(option.closest("[data-slot=select-content]")).toHaveClass(
      "max-w-(--radix-select-content-available-width)",
    );
  });
});
