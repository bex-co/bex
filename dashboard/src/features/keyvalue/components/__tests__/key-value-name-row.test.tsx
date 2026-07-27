import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KeyValueNameRow } from "@/features/keyvalue/components/key-value-name-row";
import type { KeyValueView } from "@/features/keyvalue/types";

const rename = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-rename-key-value", () => ({
  useRenameKeyValue: () => ({ rename, busy: false }),
}));

const keyValue: KeyValueView = {
  id: "red-stable",
  name: "old-name",
  status: "available",
  plan: "starter",
  version: "8",
  createdAt: null,
  externalHost: null,
  public: false,
  suspended: false,
  region: null,
};

beforeEach(() => {
  rename.mockReset();
  rename.mockResolvedValue(true);
});

describe("KeyValueNameRow", () => {
  it("renders read-only until the pencil, then renames without touching the id", async () => {
    const user = userEvent.setup();
    const onRenamed = vi.fn();
    render(<KeyValueNameRow keyValue={keyValue} onRenamed={onRenamed} />);

    const input = screen.getByLabelText("Name");
    expect(input).toHaveValue("old-name");
    expect(input).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();

    await user.click(screen.getByRole("button", { name: "Edit Key Value name" }));
    expect(input).toBeEnabled();
    await user.clear(input);
    await user.type(input, "new-name");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(rename).toHaveBeenCalledWith("red-stable", "new-name");
    expect(onRenamed).toHaveBeenCalledOnce();
  });

  it("blocks an invalid name before it reaches the API", async () => {
    const user = userEvent.setup();
    render(<KeyValueNameRow keyValue={keyValue} onRenamed={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Edit Key Value name" }));
    const input = screen.getByLabelText("Name");
    await user.clear(input);
    await user.type(input, "Bad Name");

    expect(screen.getByText(/lowercase letters/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(rename).not.toHaveBeenCalled();
  });
});
