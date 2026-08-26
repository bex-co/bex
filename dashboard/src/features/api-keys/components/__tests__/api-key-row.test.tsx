import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiKeyRow } from "@/features/api-keys/components/api-key-row";
import type { ApiKeyView } from "@/features/api-keys/types";
import { hydrateAcrossBoundary } from "@/test/hydration";

const entry: ApiKeyView = {
  id: "key-1",
  name: "deploy-agent",
  createdAt: "2026-07-01T00:00:00Z",
  createdBy: "user:minter",
  lastUsedAt: "2026-07-05T00:00:00Z",
};

beforeEach(() => {
  vi.setSystemTime(new Date("2026-07-08T00:00:00Z"));
});

describe("ApiKeyRow — metadata columns (w4/m13/t003)", () => {
  it("shows created-by and a relative last-used age", () => {
    render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={vi.fn()} revoking={false} />
        </tbody>
      </table>,
    );
    expect(screen.getByText("user:minter")).toBeInTheDocument();
    expect(screen.getByText("3d")).toBeInTheDocument(); // 2026-07-05 → 2026-07-08
  });

  it("renders a 'Never' last-used and an em dash created-by when the metadata is absent", () => {
    render(
      <table>
        <tbody>
          <ApiKeyRow
            entry={{ ...entry, createdBy: null, lastUsedAt: null }}
            onRevoke={vi.fn()}
            revoking={false}
          />
        </tbody>
      </table>,
    );
    expect(screen.getByText("Never")).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
  });

  // w6/m102: the relative ages in this row are rendered during the settings
  // route's blocking SSR pass, then again at hydration against a fresh
  // Date.now(). A bucket boundary crossed in between ("59m" → "1h" here) used to
  // surface as React error #418; RelativeAge carries the guard.
  it("hydrates across a relative-age bucket boundary without a React #418", () => {
    const lastUsedAt = "2026-07-08T00:00:00Z";
    const row = (
      <table>
        <tbody>
          <ApiKeyRow
            entry={{ ...entry, lastUsedAt }}
            onRevoke={vi.fn()}
            revoking={false}
          />
        </tbody>
      </table>
    );

    const { html, recovered } = hydrateAcrossBoundary(row, {
      serverNow: Date.parse(lastUsedAt) + 59 * 60_000,
      clientNow: Date.parse(lastUsedAt) + 60 * 60_000,
    });
    expect(html).toContain(">59m<");
    expect(recovered).toEqual([]);
  });
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
    await user.click(
      within(dialog).getAllByRole("button", { name: "Revoke" })[0],
    );

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
