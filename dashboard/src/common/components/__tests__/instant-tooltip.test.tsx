import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { InstantTooltip } from "@/common/components/instant-tooltip";
import { formatTimeAgo } from "@/common/lib/format";
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

  // Render's TimestampTooltipContent (w4/716). vitest runs in UTC, the zone
  // whose card has no second UTC row.
  it("reveals Render's rows on hover, with no UTC row for a viewer in UTC", async () => {
    const user = userEvent.setup();
    render(
      <InstantTooltip value={INSTANT}>Deployed 3 hours ago</InstantTooltip>,
    );

    await user.hover(screen.getByText("Deployed 3 hours ago"));

    const tooltip = await screen.findByRole("tooltip");
    const rows = [...tooltip.querySelectorAll("dt")].map((dt) => [
      dt.textContent,
      dt.nextElementSibling?.textContent,
    ]);
    expect(rows).toEqual([
      ["UTC:", "Jul 16 12:01:30 AM"],
      ["Timestamp:", "1784160090000"],
    ]);
  });

  it("copies each row's value as Render writes it", async () => {
    const user = userEvent.setup();
    render(
      <InstantTooltip value="2026-07-16T00:01:30.042Z">
        Deployed
      </InstantTooltip>,
    );
    await user.hover(screen.getByText("Deployed"));
    const tooltip = await screen.findByRole("tooltip");

    await user.click(within(tooltip).getByRole("button", { name: "Copy UTC" }));
    expect(await navigator.clipboard.readText()).toBe(
      "2026-07-16T00:01:30.042+00:00",
    );
    await user.click(
      within(tooltip).getByRole("button", { name: "Copy Timestamp" }),
    );
    expect(await navigator.clipboard.readText()).toBe("1784160090042");
  });

  it("adds From previous and From start, each copied, only when given", async () => {
    const user = userEvent.setup();
    render(
      <InstantTooltip
        value={INSTANT}
        previous="2026-07-16T00:01:28.8Z"
        startedAt="2026-07-16T00:00:20Z"
      >
        Deployed
      </InstantTooltip>,
    );
    await user.hover(screen.getByText("Deployed"));
    const tooltip = await screen.findByRole("tooltip");
    expect(
      [...tooltip.querySelectorAll("dt")].map((dt) => dt.textContent),
    ).toEqual(["UTC:", "Timestamp:", "From previous:", "From start:"]);
    expect(tooltip).toHaveTextContent("From previous:1.2s");
    expect(tooltip).toHaveTextContent("From start:1m 10s");

    await user.click(
      within(tooltip).getByRole("button", { name: "Copy From start" }),
    );
    expect(await navigator.clipboard.readText()).toBe("1m 10s");
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
