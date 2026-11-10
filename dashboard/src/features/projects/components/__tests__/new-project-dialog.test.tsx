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

  // w21/036: the server's label rule is checked before submit.
  it("refuses a 101-character name inline and accepts 100 CJK characters", async () => {
    const user = userEvent.setup();
    render(<NewProjectDialog onCreated={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /new project/i }));
    const name = screen.getByLabelText("Name");
    const submit = screen.getByRole("button", { name: "Create Project" });

    await user.click(name);
    await user.paste("a".repeat(101));
    expect(submit).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("up to 100 characters");

    await user.clear(name);
    await user.paste("受".repeat(100));
    expect(submit).toBeEnabled();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
