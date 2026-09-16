import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { InstantTooltip } from "@/common/components/instant-tooltip";
import { formatInstantDetails, formatTimeAgo } from "@/common/lib/format";
import { hydrateAcrossBoundary } from "@/test/hydration";

const INSTANT = "2026-07-16T00:01:30Z";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("InstantTooltip", () => {
  it("renders the caller's text in a <time> carrying the exact instant", () => {
    render(
      <InstantTooltip value={INSTANT}>Deployed 3 hours ago</InstantTooltip>,
    );
    const time = screen.getByText("Deployed 3 hours ago");
    expect(time.tagName).toBe("TIME");
    expect(time).toHaveAttribute("dateTime", INSTANT);
    // Reachable by keyboard, so the tooltip opens on focus too.
    expect(time).toHaveAttribute("tabindex", "0");
  });

  it("reveals the local, UTC, and Unix readings on hover", async () => {
    const user = userEvent.setup();
    render(
      <InstantTooltip value={INSTANT}>Deployed 3 hours ago</InstantTooltip>,
    );

    await user.hover(screen.getByText("Deployed 3 hours ago"));

    const tooltip = await screen.findByRole("tooltip");
    const details = formatInstantDetails(INSTANT)!;
    expect(tooltip).toHaveTextContent(`Local${details.local}`);
    expect(tooltip).toHaveTextContent("UTCJuly 16, 2026 at 12:01:30 AM UTC");
    expect(tooltip).toHaveTextContent("Timestamp1784160090");
  });

  it("survives elapsed text that differs between SSR and hydration (no React #418)", () => {
    // The visible text is the caller's and is expected to drift with the
    // clock — here "just now" on the SSR pass, "1 minute ago" on hydration;
    // the wrapper suppresses that one mismatch, nothing wider.
    const label = () =>
      `Deployed ${formatTimeAgo(INSTANT, { now: Date.now(), justNow: "just now" })}`;
    const Row = () => (
      <InstantTooltip value={INSTANT}>{label()}</InstantTooltip>
    );
    const { recovered } = hydrateAcrossBoundary(<Row />, {
      serverNow: Date.parse(INSTANT) + 55_000,
      clientNow: Date.parse(INSTANT) + 61_000,
    });
    expect(recovered).toEqual([]);
  });
});
