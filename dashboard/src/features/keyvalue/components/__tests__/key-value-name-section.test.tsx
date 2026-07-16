import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KeyValueNameSection } from "@/features/keyvalue/components/key-value-name-section";
import type { KeyValueView } from "@/features/keyvalue/types";

const rename = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-rename-key-value", () => ({
  useRenameKeyValue: () => ({ rename, busy: false }),
}));

const keyValue: KeyValueView = {
  id: "red-stable",
  name: "old-name",
  status: "available",
  plan: "free",
  version: "8",
  createdAt: "2026-07-14T00:00:00Z",
  externalHost: null,
  public: false,
  suspended: false,
};

beforeEach(() => {
  rename.mockReset();
  rename.mockResolvedValue(true);
});

describe("KeyValueNameSection", () => {
  it("keeps the stable id while submitting a new display name", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(<KeyValueNameSection keyValue={keyValue} onChanged={onChanged} />);

    const input = screen.getByLabelText("Name");
    await user.clear(input);
    await user.type(input, "new-name");
    await user.click(screen.getByRole("button", { name: "Save name" }));

    expect(rename).toHaveBeenCalledWith("red-stable", "new-name");
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("blocks an invalid name before it reaches the API", async () => {
    const user = userEvent.setup();
    render(<KeyValueNameSection keyValue={keyValue} onChanged={vi.fn()} />);

    const input = screen.getByLabelText("Name");
    await user.clear(input);
    await user.type(input, "Bad Name");

    expect(screen.getByText(/lowercase letters/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save name" })).toBeDisabled();
    expect(rename).not.toHaveBeenCalled();
  });
});
