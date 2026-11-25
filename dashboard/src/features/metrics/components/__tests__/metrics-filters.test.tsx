import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  ApolloClient,
  ApolloLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MetricsFilters } from "../metrics-filters";
import { DEFAULT_RANGE_PRESET } from "../../lib/range";

// w7/m427: the Metrics toolbar's range picker reaches back only as far as the
// service's workspace plan keeps metrics, as bex-api's metricsRetention
// reports it: Hobby 7 days, Pro 14, Scale and Enterprise 30 (Render's
// windows). The real hook and picker run here; only the network is faked.

// Radix Select/Dialog lean on APIs jsdom doesn't implement.
beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }
});

afterEach(cleanup);

type Upgrade = { plan: string; days: number };
type Retention = { plan: string; days: number; upgrades: Upgrade[] } | null;

const PRO: Upgrade = { plan: "pro", days: 14 };
const SCALE: Upgrade = { plan: "scale", days: 30 };

function mount(retention: Retention, { events = true } = {}) {
  const onRangeChange = vi.fn();
  const reads: Record<string, unknown>[] = [];
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: new ApolloLink(
      (operation) =>
        new Observable<ApolloLink.Result>((observer) => {
          reads.push(operation.variables);
          observer.next({
            data: {
              metricsRetention: retention && {
                __typename: "MetricsRetention",
                ...retention,
                upgrades: retention.upgrades.map((upgrade) => ({
                  __typename: "MetricsRetentionUpgrade",
                  ...upgrade,
                })),
              },
            },
          });
          observer.complete();
        }),
    ),
  });
  render(
    <ApolloProvider client={client}>
      <MetricsFilters
        resource="srv-1"
        range={DEFAULT_RANGE_PRESET}
        onRangeChange={onRangeChange}
        events={
          events
            ? {
                filter: "all",
                onFilterChange: vi.fn(),
                timelineShown: false,
                onTimelineShownChange: vi.fn(),
              }
            : undefined
        }
      />
    </ApolloProvider>,
  );
  return { onRangeChange, reads };
}

const WIDE_PRESETS = ["Last 7 days", "Last 14 days", "Last 30 days"];

/** The wide presets the open picker has disabled. */
function disabledPresets(): string[] {
  return WIDE_PRESETS.filter(
    (name) =>
      screen.getByRole("option", { name }).getAttribute("aria-disabled") ===
      "true",
  );
}

describe("MetricsFilters range picker follows the plan's metrics window", () => {
  it.each([
    {
      plan: "hobby",
      retention: { plan: "hobby", days: 7, upgrades: [PRO, SCALE] },
      disabled: ["Last 14 days", "Last 30 days"],
      hint: "Metrics are kept for 7 days on the Hobby plan. The Pro plan keeps 14 days. The Scale plan keeps 30 days.",
    },
    {
      plan: "pro",
      retention: { plan: "pro", days: 14, upgrades: [SCALE] },
      disabled: ["Last 30 days"],
      hint: "Metrics are kept for 14 days on the Pro plan. The Scale plan keeps 30 days.",
    },
    {
      plan: "scale",
      retention: { plan: "scale", days: 30, upgrades: [] },
      disabled: [],
      hint: "Metrics are kept for 30 days on the Scale plan.",
    },
  ])(
    "on $plan disables the presets wider than its window and says what each plan keeps",
    async ({ retention, disabled, hint }) => {
      const user = userEvent.setup();
      const { reads } = mount(retention);
      await waitFor(() => expect(reads).toHaveLength(1));
      expect(reads[0]).toEqual({ resource: "srv-1" });

      await user.click(screen.getByLabelText("Time range"));
      await waitFor(() => expect(screen.getByText(hint)).toBeInTheDocument());
      expect(disabledPresets()).toEqual(disabled);
    },
  );

  it("refuses a custom range that starts before a Hobby workspace's floor", async () => {
    const user = userEvent.setup();
    const { onRangeChange, reads } = mount({
      plan: "hobby",
      days: 7,
      upgrades: [PRO, SCALE],
    });
    await waitFor(() => expect(reads).toHaveLength(1));
    await user.click(screen.getByLabelText("Time range"));
    await waitFor(() => expect(disabledPresets()).toHaveLength(2));
    await user.click(screen.getByRole("option", { name: /Custom/ }));

    const DAY_MS = 24 * 60 * 60 * 1000;
    const localInput = (ms: number) => {
      const d = new Date(ms);
      const pad = (n: number) => String(n).padStart(2, "0");
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
    };
    const now = Date.now();
    fireEvent.change(screen.getByLabelText("Start"), {
      target: { value: localInput(now - 10 * DAY_MS) },
    });
    fireEvent.change(screen.getByLabelText("End"), {
      target: { value: localInput(now - 9 * DAY_MS) },
    });
    await user.click(screen.getByRole("button", { name: "Apply" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      /^The range must start on or after .+\.$/,
    );
    expect(onRangeChange).not.toHaveBeenCalled();
  });

  it("leaves every preset open where no plan applies", async () => {
    const user = userEvent.setup();
    const { reads } = mount(null);
    await waitFor(() => expect(reads).toHaveLength(1));
    await user.click(screen.getByLabelText("Time range"));
    expect(disabledPresets()).toEqual([]);
    expect(screen.queryByText(/Metrics are kept for/)).not.toBeInTheDocument();
  });
});

// w3/103: Render's cron job Metrics page has the range picker alone.
describe("MetricsFilters without event controls", () => {
  it("draws the range picker with no event filter or timeline toggle", async () => {
    const user = userEvent.setup();
    const { reads } = mount(null, { events: false });
    await waitFor(() => expect(reads).toHaveLength(1));

    expect(screen.getAllByRole("combobox")).toEqual([
      screen.getByLabelText("Time range"),
    ]);
    expect(
      screen.queryByRole("button", { name: "Show event timeline" }),
    ).not.toBeInTheDocument();
    await user.click(screen.getByLabelText("Time range"));
    expect(
      screen.getByRole("option", { name: "Last 12 hours" }),
    ).toBeInTheDocument();
  });

  it("keeps the event filter and timeline toggle beside it otherwise", async () => {
    const { reads } = mount(null);
    await waitFor(() => expect(reads).toHaveLength(1));

    expect(screen.getAllByRole("combobox")).toHaveLength(2);
    expect(
      screen.getByRole("button", { name: "Show event timeline" }),
    ).toBeInTheDocument();
  });
});
