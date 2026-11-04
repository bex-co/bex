import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NewProjectDialog } from "@/features/projects/components/new-project-dialog";

const create = vi.fn();

vi.mock("@/features/projects/hooks/use-create-project", () => ({
  useCreateProject: () => ({ create, busy: false }),
}));

beforeEach(() => {
  create.mockReset().mockResolvedValue("prj-new");
});

// w4/m264: Render's create-project asks for the first environment's name.
describe("NewProjectDialog", () => {
  it("creates the project with a Production first environment by default", async () => {
    const user = userEvent.setup();
    const onCreated = vi.fn();
    render(<NewProjectDialog onCreated={onCreated} />);

    await user.click(screen.getByRole("button", { name: /new project/i }));
    expect(screen.getByLabelText("First environment name")).toHaveValue(
      "Production",
    );
    await user.type(screen.getByLabelText("Name"), "shop");
    await user.click(screen.getByRole("button", { name: "Create Project" }));

    expect(create).toHaveBeenCalledWith("shop", "Production");
    expect(onCreated).toHaveBeenCalledWith("prj-new");
  });

  it("will not submit with a blank environment name", async () => {
    const user = userEvent.setup();
    render(<NewProjectDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /new project/i }));
    await user.type(screen.getByLabelText("Name"), "shop");
    await user.clear(screen.getByLabelText("First environment name"));

    expect(
      screen.getByRole("button", { name: "Create Project" }),
    ).toBeDisabled();
  });
});
