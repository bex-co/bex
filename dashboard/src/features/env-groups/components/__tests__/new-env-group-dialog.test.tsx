import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const createGroup = vi.fn();

vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroupMutations: () => ({ createGroup, busy: false }),
}));

import { NewEnvGroupDialog } from "@/features/env-groups/components/new-env-group-dialog";

beforeEach(() => {
  createGroup.mockReset().mockResolvedValue("eg-new");
});

describe("NewEnvGroupDialog", () => {
  it("blocks a blank name, then returns the created group id", async () => {
    const onCreated = vi.fn();
    const user = userEvent.setup();
    render(<NewEnvGroupDialog open onCreated={onCreated} />);

    const submit = screen.getByRole("button", {
      name: "Create Environment Group",
    });
    expect(submit).toBeDisabled();

    await user.type(screen.getByLabelText("Group name"), "   ");
    expect(submit).toBeDisabled();
    expect(createGroup).not.toHaveBeenCalled();

    await user.clear(screen.getByLabelText("Group name"));
    await user.type(screen.getByLabelText("Group name"), "Shared production");
    await user.click(submit);

    expect(createGroup).toHaveBeenCalledWith("Shared production");
    expect(onCreated).toHaveBeenCalledWith("eg-new");
  });

  it("stays open and does not navigate when creation fails", async () => {
    createGroup.mockResolvedValue(null);
    const onCreated = vi.fn();
    const user = userEvent.setup();
    render(<NewEnvGroupDialog open onCreated={onCreated} />);

    await user.type(screen.getByLabelText("Group name"), "shared");
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );

    expect(onCreated).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("Group name")).toHaveValue("shared");
  });
});
