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
  it("fires the mutation with the newly selected policy", async () => {
    // pointerEventsCheck off: Radix Select styles the trigger/options in ways
    // userEvent's pointer-events guard trips over in jsdom.
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValueMaxmemoryPolicySection id="red-abc" />);

    // Nothing to save until the policy actually changes.
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();

    await user.click(screen.getByRole("combobox"));
    await user.click(screen.getByRole("option", { name: "volatile-lru" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(save).toHaveBeenCalledWith("volatile-lru");
  });

  it("re-disables Save after resetting the draft to the current policy", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValueMaxmemoryPolicySection id="red-abc" />);

    await user.click(screen.getByRole("combobox"));
    await user.click(screen.getByRole("option", { name: "noeviction" }));
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(save).not.toHaveBeenCalled();
  });
});
