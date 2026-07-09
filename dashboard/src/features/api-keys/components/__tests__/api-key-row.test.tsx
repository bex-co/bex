import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiKeyRow } from "@/features/api-keys/components/api-key-row";
import type { ApiKeyView } from "@/features/api-keys/types";

const entry: ApiKeyView = {
  id: "key-1",
  name: "deploy-agent",
  createdAt: "2026-07-01T00:00:00Z",
};

beforeEach(() => {
  vi.setSystemTime(new Date("2026-07-08T00:00:00Z"));
});

describe("ApiKeyRow — revoke with confirmation (w4/m8/t002)", () => {
  it("does not revoke on the row button alone — a confirmation dialog gates it", async () => {
    const onRevoke = vi.fn();
    const user = userEvent.setup();
    render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={onRevoke} revoking={false} />
        </tbody>
      </table>,
    );

    await user.click(screen.getByRole("button", { name: "Revoke" }));
    expect(onRevoke).not.toHaveBeenCalled();
    expect(await screen.findByText("Revoke deploy-agent?")).toBeInTheDocument();
  });

  it("revokes only after the confirmation is accepted", async () => {
    const onRevoke = vi.fn().mockResolvedValue(true);
    const user = userEvent.setup();
    render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={onRevoke} revoking={false} />
        </tbody>
      </table>,
    );

    await user.click(screen.getByRole("button", { name: "Revoke" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getAllByRole("button", { name: "Revoke" })[0]);

    expect(onRevoke).toHaveBeenCalledWith("key-1", "deploy-agent");
  });

  it("cancel closes the dialog without revoking", async () => {
    const onRevoke = vi.fn();
    const user = userEvent.setup();
    render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={onRevoke} revoking={false} />
        </tbody>
      </table>,
    );

    await user.click(screen.getByRole("button", { name: "Revoke" }));
    await screen.findByRole("alertdialog");
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onRevoke).not.toHaveBeenCalled();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("disables the row control while this row is revoking", () => {
    render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={vi.fn()} revoking={true} />
        </tbody>
      </table>,
    );
    expect(screen.getByRole("button", { name: "Revoke" })).toBeDisabled();
  });
});
