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
