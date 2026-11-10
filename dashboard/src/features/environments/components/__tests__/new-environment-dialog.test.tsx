import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NewEnvironmentDialog } from "@/features/environments/components/new-environment-dialog";

const create = vi.fn();
vi.mock("@/features/environments/hooks/use-create-environment", () => ({
  useCreateEnvironment: () => ({ create, busy: false }),
}));

beforeEach(() => {
  create.mockReset();
});

describe("NewEnvironmentDialog contextual create outcome", () => {
  it("closes after the owning Project query accepts the returned environment id", async () => {
    create.mockResolvedValue("env-returned-id");
    const user = userEvent.setup();
    render(<NewEnvironmentDialog projectId="prj-owner" />);

    await user.click(screen.getByRole("button", { name: "New Environment" }));
    await user.type(screen.getByLabelText("Name"), "Production");
    await user.click(
      screen.getByRole("button", { name: "Create Environment" }),
    );

    expect(create).toHaveBeenCalledWith("Production");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("keeps the owning Project dialog and name in place when create fails", async () => {
    create.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<NewEnvironmentDialog projectId="prj-owner" />);

    await user.click(screen.getByRole("button", { name: "New Environment" }));
    await user.type(screen.getByLabelText("Name"), "Production");
    await user.click(
      screen.getByRole("button", { name: "Create Environment" }),
    );

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveValue("Production");
  });
});

// w21/036: the server's label rule is checked before submit.
describe("NewEnvironmentDialog name rule", () => {
  it("refuses a 101-character name inline and accepts 100", async () => {
    const user = userEvent.setup();
    render(<NewEnvironmentDialog projectId="prj-owner" />);
    await user.click(screen.getByRole("button", { name: "New Environment" }));
    const name = screen.getByLabelText("Name");
    const submit = screen.getByRole("button", { name: "Create Environment" });

    await user.click(name);
    await user.paste("a".repeat(101));
    expect(submit).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("up to 100 characters");

    await user.clear(name);
    await user.paste("受".repeat(100));
    expect(submit).toBeEnabled();
  });
});
