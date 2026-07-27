import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KeyValueMaxmemoryPolicySection } from "@/features/keyvalue/components/key-value-maxmemory-policy-section";

const save = vi.fn();
let hookState = {
  policy: "allkeys-lru",
  loading: false,
  saving: false,
  save,
};

vi.mock(
  "@/features/keyvalue/hooks/use-set-key-value-maxmemory-policy",
  () => ({
    useSetKeyValueMaxmemoryPolicy: () => hookState,
  }),
);

beforeEach(() => {
  save.mockReset();
  save.mockResolvedValue(true);
  hookState = { policy: "allkeys-lru", loading: false, saving: false, save };
});

describe("KeyValueMaxmemoryPolicySection", () => {
  it("is read-only until the pencil, then fires the mutation with the new policy", async () => {
    // pointerEventsCheck off: Radix Select styles the trigger/options in ways
    // userEvent's pointer-events guard trips over in jsdom.
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValueMaxmemoryPolicySection id="red-abc" />);

    // Edit-in-place: the current policy sits in a disabled select, no Save yet.
    const select = screen.getByRole("combobox", { name: "Maxmemory policy" });
    expect(select).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();

    await user.click(
      screen.getByRole("button", { name: "Edit maxmemory policy" }),
    );
    // Editable now; Save disabled until the policy actually changes.
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();

    await user.click(screen.getByRole("combobox", { name: "Maxmemory policy" }));
    await user.click(screen.getByRole("option", { name: "volatile-lru" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(save).toHaveBeenCalledWith("volatile-lru");
  });

  it("cancel restores the current policy and leaves edit mode without saving", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValueMaxmemoryPolicySection id="red-abc" />);

    await user.click(
      screen.getByRole("button", { name: "Edit maxmemory policy" }),
    );
    await user.click(screen.getByRole("combobox", { name: "Maxmemory policy" }));
    await user.click(screen.getByRole("option", { name: "noeviction" }));
    expect(screen.getByRole("button", { name: "Save changes" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    // Back to read-only: the pencil returns and there is no Save button.
    expect(
      screen.getByRole("button", { name: "Edit maxmemory policy" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();
    expect(save).not.toHaveBeenCalled();
  });
});
