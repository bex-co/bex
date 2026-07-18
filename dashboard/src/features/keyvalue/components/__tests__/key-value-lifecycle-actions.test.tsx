import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KeyValueLifecycleActions } from "@/features/keyvalue/components/key-value-lifecycle-actions";
import type { KeyValueView } from "@/features/keyvalue/types";

const run = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-key-value-lifecycle", () => ({
  useKeyValueLifecycle: () => ({ pending: null, run }),
}));

const KEY_VALUE: KeyValueView = {
  id: "red-sessions",
  name: "sessions-cache",
  status: "available",
  plan: "starter",
  version: "8",
  createdAt: null,
  externalHost: null,
  public: false,
  suspended: false,
};

const CONFIRM_PHRASE = "sudo suspend key value sessions-cache";

beforeEach(() => {
  run.mockReset();
  run.mockResolvedValue(true);
});

describe("KeyValueLifecycleActions", () => {
  it("requires Render's full sudo phrase before suspending", async () => {
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(
      <KeyValueLifecycleActions keyValue={KEY_VALUE} onChanged={onChanged} />,
    );

    await user.click(
      screen.getByRole("button", { name: "Suspend Key Value Instance" }),
    );

    const dialog = await screen.findByRole("alertdialog");
    const confirm = within(dialog).getByRole("button", {
      name: "Suspend Key Value Instance",
    });
    const input = within(dialog).getByLabelText(
      `Type ${CONFIRM_PHRASE} below to confirm`,
    );

    expect(confirm).toBeDisabled();

    await user.type(input, "sessions-cache");
    expect(confirm).toBeDisabled();

    await user.clear(input);
    await user.type(input, CONFIRM_PHRASE);
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(run).toHaveBeenCalledWith(
      "suspend",
      "red-sessions",
      "sessions-cache",
    );
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("resumes immediately without the destructive confirmation", async () => {
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(
      <KeyValueLifecycleActions
        keyValue={{ ...KEY_VALUE, suspended: true }}
        onChanged={onChanged}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Resume Key Value Instance" }),
    );

    expect(run).toHaveBeenCalledWith(
      "resume",
      "red-sessions",
      "sessions-cache",
    );
    expect(onChanged).toHaveBeenCalledOnce();
  });
});
