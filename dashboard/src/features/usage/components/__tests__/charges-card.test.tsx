// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { describe, expect, it, vi } from "vitest";
import { act, type ReactElement } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { fireEvent, render, screen } from "@testing-library/react";
import { ChargesCard } from "@/features/usage/components/charges-card";
import {
  billingWindow,
  groupByCategory,
  projectMonthEnd,
  projectPeriodEnd,
} from "@/features/usage/lib/charges";
import type {
  EstimatedCost,
  ResourceEstimate,
} from "@/features/usage/hooks/use-usage";

function resource(over: Partial<ResourceEstimate> = {}): ResourceEstimate {
  return {
    serviceId: "srv-a",
    serviceName: "api",
    resourceKind: "service",
    costUsd: "4.90",
    charges: [
      {
        kind: "instance_seconds",
        tier: "starter",
        unit: "hr",
        rateUsd: "0.006713",
        quantity: "730.00",
        costUsd: "4.90",
      },
    ],
    ...over,
  };
}

function estimate(
  resources: ResourceEstimate[],
  totalUsd = "4.90",
): EstimatedCost {
  return { totalUsd, resources };
}

// Mid-month, so the projection is exercised: 15 days into a 31-day July.
const MID_JULY = new Date(2026, 6, 16, 0, 0, 0);

describe("groupByCategory", () => {
  it("orders known categories and sorts resources by cost within one", () => {
    const groups = groupByCategory([
      resource({
        serviceId: "kv-1",
        resourceKind: "key_value",
        costUsd: "7.00",
      }),
      resource({ serviceId: "srv-cheap", costUsd: "1.00" }),
      resource({ serviceId: "srv-dear", costUsd: "9.00" }),
    ]);

    expect(groups.map((g) => g.key)).toEqual(["service", "key_value"]);
    expect(groups[0]!.resources.map((r) => r.serviceId)).toEqual([
      "srv-dear",
      "srv-cheap",
    ]);
    expect(groups[0]!.totalUsd).toBeCloseTo(10);
  });

  it("keeps a resource kind the frontend does not know yet", () => {
    // It still bills, so hiding it would make the categories disagree with the
    // total printed underneath them.
    const groups = groupByCategory([
      resource({ serviceId: "x-1", resourceKind: "quantum_widget" }),
    ]);
    expect(groups.map((g) => g.key)).toEqual(["quantum_widget"]);
  });
});

describe("projectMonthEnd", () => {
  it("scales month-to-date spend by the fraction of the month elapsed", () => {
    // 15 of 31 days gone with $10 spent ⇒ about $20.67 by month end.
    expect(projectMonthEnd(10, MID_JULY)).toBeCloseTo(20.67, 1);
  });

  it("returns null before anything has been spent", () => {
    expect(projectMonthEnd(0, MID_JULY)).toBeNull();
  });

  it("returns null at the very start of a month, where the ratio explodes", () => {
    expect(projectMonthEnd(10, new Date(2026, 6, 1, 0, 0, 0))).toBeNull();
  });
});

describe("projectPeriodEnd", () => {
  // The w6/050 live shape: a subscription period anchored on the 16th. $30.01
  // over 11 of its 31 days projects to ~$84.57 — the calendar-month math
  // (26 of 31 August days elapsed) said $35.78, understating it ~2.4×.
  const AUG_16 = new Date(2026, 7, 16);
  const SEP_16 = new Date(2026, 8, 16);

  it("scales spend by the elapsed fraction of the given window", () => {
    expect(
      projectPeriodEnd(30.01, new Date(2026, 7, 27), AUG_16, SEP_16),
    ).toBeCloseTo(84.57, 1);
  });

  it("handles a window spanning two months with now in the second", () => {
    // Sep 5 is 20 days into the Aug 16 → Sep 16 window.
    expect(
      projectPeriodEnd(30.01, new Date(2026, 8, 5), AUG_16, SEP_16),
    ).toBeCloseTo(46.52, 1);
  });

  it("matches the calendar-month projection when the window is the month", () => {
    expect(
      projectPeriodEnd(
        10,
        MID_JULY,
        new Date(2026, 6, 1),
        new Date(2026, 7, 1),
      ),
    ).toBe(projectMonthEnd(10, MID_JULY));
  });

  it("returns null at or before the window start, where the ratio explodes", () => {
    expect(projectPeriodEnd(30.01, AUG_16, AUG_16, SEP_16)).toBeNull();
    expect(
      projectPeriodEnd(30.01, new Date(2026, 7, 10), AUG_16, SEP_16),
    ).toBeNull();
  });

  it("returns null once the window has closed — nothing left to project", () => {
    expect(projectPeriodEnd(30.01, SEP_16, AUG_16, SEP_16)).toBeNull();
    expect(
      projectPeriodEnd(30.01, new Date(2026, 8, 20), AUG_16, SEP_16),
    ).toBeNull();
  });

  it("returns null for an empty or inverted window", () => {
    expect(
      projectPeriodEnd(30.01, new Date(2026, 7, 27), AUG_16, AUG_16),
    ).toBeNull();
    expect(
      projectPeriodEnd(30.01, new Date(2026, 7, 27), SEP_16, AUG_16),
    ).toBeNull();
  });
});

describe("billingWindow", () => {
  it("parses a valid RFC3339 pair", () => {
    const w = billingWindow("2026-08-16T00:00:00Z", "2026-09-16T00:00:00Z");
    expect(w).not.toBeNull();
    expect(w!.start.toISOString()).toBe("2026-08-16T00:00:00.000Z");
    expect(w!.end.toISOString()).toBe("2026-09-16T00:00:00.000Z");
  });

  it("returns null when either bound is missing or unparseable", () => {
    expect(billingWindow("", "2026-09-16T00:00:00Z")).toBeNull();
    expect(billingWindow("2026-08-16T00:00:00Z", "")).toBeNull();
    expect(billingWindow(null, null)).toBeNull();
    expect(billingWindow("not-a-date", "2026-09-16T00:00:00Z")).toBeNull();
  });
});

describe("ChargesCard", () => {
  it("shows categories collapsed, and drills to resource then charge line", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource()])}
        invoicedUsd={null}
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    // Collapsed: the category and its total, nothing below it.
    expect(screen.getByText("Services")).toBeInTheDocument();
    expect(screen.queryByText("api")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Services/ }));
    expect(screen.getByText("api")).toBeInTheDocument();
    expect(screen.queryByText("Compute")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /api/ }));
    expect(screen.getByText("Compute")).toBeInTheDocument();
    // Rate and quantity share a unit so the arithmetic reads correctly.
    expect(screen.getByText("$0.006713/hr")).toBeInTheDocument();
    expect(screen.getByText("730.00 hr")).toBeInTheDocument();
  });

  it("expands and collapses every category at once", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource()])}
        invoicedUsd={null}
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Expand all" }));
    expect(screen.getByText("api")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Collapse all" }));
    expect(screen.queryByText("api")).not.toBeInTheDocument();
  });

  it("falls back to the resource id when it has no display name", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource({ serviceName: "" })])}
        invoicedUsd={null}
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /Services/ }));
    expect(screen.getByText("srv-a")).toBeInTheDocument();
  });

  it("labels a zero rate as included rather than as $0/hr", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([
          resource({
            costUsd: "0.00",
            charges: [
              {
                kind: "instance_seconds",
                tier: "free",
                unit: "hr",
                rateUsd: "0",
                quantity: "730.00",
                costUsd: "0.00",
              },
            ],
          }),
        ])}
        invoicedUsd={null}
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Expand all" }));
    fireEvent.click(screen.getByRole("button", { name: /api/ }));
    expect(screen.getByText("Included")).toBeInTheDocument();
  });

  it("totals the estimate and projects month end when there is no invoice", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource()], "10.00")}
        invoicedUsd={null}
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    expect(screen.getByText("Total month to date")).toBeInTheDocument();
    expect(screen.getByText(/Projected total for July/)).toBeInTheDocument();
    expect(
      screen.getByText(/priced from the bex rate sheet/),
    ).toBeInTheDocument();
  });

  it("sums the estimate total from the categories shown, not totalUsd", () => {
    // The backend rounds the raw total once and the tree rounds per resource,
    // so the two disagree by a cent. Whatever the card prints must equal what
    // a reader gets by adding up the rows above it.
    render(
      <ChargesCard
        estimatedCost={estimate(
          [
            resource({ serviceId: "srv-a", costUsd: "1.23" }),
            resource({
              serviceId: "db-a",
              resourceKind: "postgres",
              costUsd: "2.65",
            }),
          ],
          // Deliberately inconsistent with the parts, as real rounding is.
          "3.87",
        )}
        invoicedUsd={null}
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    expect(screen.getByText("$3.88 USD")).toBeInTheDocument();
    expect(screen.queryByText("$3.87 USD")).not.toBeInTheDocument();
  });

  it("prefers Stripe's real amount over the estimate when one exists", () => {
    // The invoice is authoritative; the estimate is only a stand-in for
    // workspaces that have no subscription to price them.
    render(
      <ChargesCard
        estimatedCost={estimate([resource()], "10.00")}
        invoicedUsd="12.34"
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    expect(screen.getByText("$12.34 USD")).toBeInTheDocument();
    expect(screen.queryByText("$10.00 USD")).not.toBeInTheDocument();
    expect(screen.getByText(/as rated by Stripe/)).toBeInTheDocument();
  });

  // w6/m98: the production shape. A $1,000 credit grant absorbed the whole
  // period, so Stripe's invoice total was $0.00 while the tree summed to
  // $74.78 — and the headline reported the $0.00.
  it("shows the gross charge as the total and the credited amount due beneath it", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource({ costUsd: "74.74" })], "74.74")}
        invoicedUsd="74.78"
        amountDueUsd="0.00"
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    expect(screen.getByText("$74.78 USD")).toBeInTheDocument();
    expect(screen.getByText("Amount due after credits")).toBeInTheDocument();
    expect(screen.getByText("$0.00 USD")).toBeInTheDocument();
  });

  it("does not repeat the total as an amount due when nothing was credited", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource()], "4.90")}
        invoicedUsd="12.34"
        amountDueUsd="12.34"
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    expect(screen.getByText("$12.34 USD")).toBeInTheDocument();
    expect(
      screen.queryByText("Amount due after credits"),
    ).not.toBeInTheDocument();
  });

  it("keeps the estimate as the total while Stripe has rated nothing", () => {
    // A zero from Stripe means its meter events have not landed yet, not that
    // the period was free. Printing that zero over a nonzero tree is the
    // contradiction the page must never show.
    render(
      <ChargesCard
        estimatedCost={estimate([resource({ costUsd: "74.74" })], "74.74")}
        invoicedUsd="0.00"
        amountDueUsd="0.00"
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    expect(screen.getByText("$74.74 USD")).toBeInTheDocument();
    expect(screen.queryByText("$0.00 USD")).not.toBeInTheDocument();
    expect(
      screen.getByText(/priced from the bex rate sheet/),
    ).toBeInTheDocument();
  });

  // w6/050: Stripe's rated total covers the subscription period (anchored on
  // the subscription's day-of-month), not the calendar month, so its
  // projection must use that window.
  it("projects a rated total over its subscription period, not the calendar month", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource()], "10.00")}
        invoicedUsd="12.34"
        ratedPeriodStart={new Date(2026, 6, 6).toISOString()}
        ratedPeriodEnd={new Date(2026, 7, 6).toISOString()}
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    // July 6 → Aug 6 is 31 days with 10 elapsed: $12.34 × 31/10 ⇒ $38.25.
    expect(
      screen.getByText("Projected for this billing period"),
    ).toBeInTheDocument();
    expect(screen.getByText("$38.25 USD")).toBeInTheDocument();
    // The calendar-month math (15 of July's 31 days) would have said $25.50
    // and named a month the window only partly covers.
    expect(screen.queryByText("$25.50 USD")).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Projected total for July/),
    ).not.toBeInTheDocument();
  });

  it("does not project a rated total whose period bounds are unknown", () => {
    // Projecting over the calendar month would be wrong by construction, so
    // an unknowable window means no projection at all.
    render(
      <ChargesCard
        estimatedCost={estimate([resource()], "10.00")}
        invoicedUsd="12.34"
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    expect(screen.queryByText(/Projected/)).not.toBeInTheDocument();
  });

  it("keeps the calendar-month window and label for the estimate fallback", () => {
    // The category sum accrues from the 1st, so the month is its right
    // window even when Stripe period bounds happen to be present.
    render(
      <ChargesCard
        estimatedCost={estimate([resource()], "10.00")}
        invoicedUsd={null}
        ratedPeriodStart={new Date(2026, 6, 6).toISOString()}
        ratedPeriodEnd={new Date(2026, 7, 6).toISOString()}
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    // $4.90 (the category sum) × 31/15 ⇒ $10.13.
    expect(screen.getByText(/Projected total for July/)).toBeInTheDocument();
    expect(screen.getByText("$10.13 USD")).toBeInTheDocument();
    expect(
      screen.queryByText("Projected for this billing period"),
    ).not.toBeInTheDocument();
  });

  it("does not project a month that has already ended", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource()], "10.00")}
        invoicedUsd={null}
        loading={false}
        period="2026-05"
        now={MID_JULY}
      />,
    );

    expect(screen.getByText("Total for the period")).toBeInTheDocument();
    expect(screen.queryByText(/Projected total/)).not.toBeInTheDocument();
  });

  it("shows an empty state, still with a total, when nothing was metered", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([], "0.00")}
        invoicedUsd={null}
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );

    expect(screen.getByText("No usage in this period.")).toBeInTheDocument();
    expect(screen.getByText("$0.00 USD")).toBeInTheDocument();
  });

  // The Charges card is the trust surface for real money, so it must never
  // assert one claim about a dollar figure and then contradict it. Before
  // w10/m11/t001, `invoicedUsd == null` conflated "this workspace has no Stripe
  // pricing" with "the number has not arrived yet", so the first paint read
  // "An estimate, not an invoice." and then swapped to "the amount Stripe will
  // invoice." moments later.
  it("does not claim estimate-or-invoice while the invoiced total is still loading", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource()])}
        invoicedUsd={null}
        loading
        period=""
        now={MID_JULY}
      />,
    );

    expect(
      screen.queryByText(/an estimate, not an invoice/i),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/as rated by Stripe/i)).not.toBeInTheDocument();
    // It still says the one thing that is already true.
    expect(screen.getByText(/accrued so far this period/i)).toBeInTheDocument();
  });

  it("settles on the estimate wording once loading finishes with no invoiced total", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource()])}
        invoicedUsd={null}
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );
    expect(
      screen.getByText(/an estimate, not an invoice/i),
    ).toBeInTheDocument();
  });

  it("settles on the invoiced wording once an invoiced total resolves", () => {
    render(
      <ChargesCard
        estimatedCost={estimate([resource()])}
        invoicedUsd="12.34"
        loading={false}
        period=""
        now={MID_JULY}
      />,
    );
    expect(screen.getByText(/as rated by Stripe/i)).toBeInTheDocument();
    expect(
      screen.queryByText(/an estimate, not an invoice/i),
    ).not.toBeInTheDocument();
  });
});

/**
 * SSR-render `serverNode`, then hydrate that exact markup as `clientNode`, and
 * report the server HTML, the settled DOM text, and any React #418s
 * (`recovered`). Distinct nodes simulate `now = new Date()` being evaluated
 * independently in each pass — the production path every test above bypasses
 * by injecting a fixed `now` (w6/049).
 */
function ssrThenHydrate(
  serverNode: ReactElement,
  clientNode: ReactElement = serverNode,
) {
  const html = renderToString(serverNode);
  const container = document.createElement("div");
  container.innerHTML = html;
  document.body.appendChild(container);
  const recovered: unknown[] = [];
  // React logs the mismatch as well; onRecoverableError is the assertable
  // channel, so keep the console quiet.
  const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  let root: ReturnType<typeof hydrateRoot> | undefined;
  act(() => {
    root = hydrateRoot(container, clientNode, {
      onRecoverableError: (e) => recovered.push(e),
    });
  });
  const afterHydrate = container.textContent ?? "";
  act(() => root?.unmount());
  container.remove();
  errSpy.mockRestore();
  return { html, afterHydrate, recovered };
}

describe("ChargesCard coverage caveat (w4/048)", () => {
  const baseProps = {
    estimatedCost: estimate([resource()]),
    invoicedUsd: null,
    loading: false,
    period: "",
    now: MID_JULY,
  };

  it("caveats a partial estimate, naming the coverage boundary and degraded meters", () => {
    render(
      <ChargesCard
        {...baseProps}
        coverage={{
          state: "partial",
          through: "2026-09-01T00:00:00Z",
          degradedSources: ["direct", "http", "instance"],
        }}
      />,
    );

    const caveat = screen.getByText("Partial data");
    expect(caveat).toBeInTheDocument();
    // The tooltip discloses the date the estimate is complete through and the
    // degraded meters, so the dollar figure isn't presented as authoritative.
    const title = caveat.getAttribute("title") ?? "";
    expect(title).toContain("2026-09-01");
    expect(title).toContain("direct, http, instance");
  });

  it("shows no caveat when metering is complete", () => {
    render(
      <ChargesCard
        {...baseProps}
        coverage={{ state: "complete", through: "", degradedSources: [] }}
      />,
    );
    expect(screen.queryByText("Partial data")).not.toBeInTheDocument();
  });

  it("stays silent for an indeterminate read with nothing to disclose", () => {
    render(
      <ChargesCard
        {...baseProps}
        coverage={{ state: "unknown", through: "", degradedSources: [] }}
      />,
    );
    expect(screen.queryByText("Partial data")).not.toBeInTheDocument();
  });

  it("omits the caveat entirely when coverage is not provided (back-compat)", () => {
    render(<ChargesCard {...baseProps} />);
    expect(screen.queryByText("Partial data")).not.toBeInTheDocument();
  });
});

describe("ChargesCard across the SSR/hydration boundary (w6/049)", () => {
  const props = {
    estimatedCost: estimate([resource()]),
    invoicedUsd: null,
    loading: false,
    period: "",
  };

  it("emits no projection during SSR and hydrates cleanly across two clock reads", () => {
    const { html, afterHydrate, recovered } = ssrThenHydrate(
      <ChargesCard {...props} now={MID_JULY} />,
      <ChargesCard {...props} now={new Date(2026, 6, 16, 1, 0, 0)} />,
    );

    // The SSR pass bakes no clock-derived projection into the markup…
    expect(html).not.toContain("Projected");
    // …so the hour between the two clock reads — which would price the
    // projection at $10.13 server-side vs $10.10 client-side — cannot
    // mismatch as text (React error #418).
    expect(recovered).toHaveLength(0);
    // The row appears after hydration, priced from the client's clock only.
    expect(afterHydrate).toContain("Projected total for July");
    expect(afterHydrate).toContain("$10.10 USD");
  });

  it("covers the real default-clock path: each pass reads its own new Date()", () => {
    // No `now` prop at all — the default parameter runs twice, at genuinely
    // different instants, exactly as in production.
    const { html, afterHydrate, recovered } = ssrThenHydrate(
      <ChargesCard {...props} />,
    );

    expect(html).not.toContain("Projected");
    expect(recovered).toHaveLength(0);
    expect(afterHydrate).toContain("Projected total for");
  });

  it("tolerates a month boundary between the SSR and hydration clock reads", () => {
    // The total-row label flips from "month to date" to "for the period" at
    // the boundary. It carries suppressHydrationWarning rather than a mount
    // gate (gating would flash the wrong label on every load to guard a
    // sub-second once-a-month race that the real page — which always hydrates
    // with period="" — cannot even reach), so the divergence must not raise a
    // #418. Under suppression React keeps the server's label rather than
    // patching it, so no exact-label assertion here — the absence of a
    // recoverable error is the guarantee.
    const juneProps = { ...props, period: "2026-06" };
    const { afterHydrate, recovered } = ssrThenHydrate(
      <ChargesCard {...juneProps} now={new Date(2026, 5, 30, 23, 59, 59)} />,
      <ChargesCard {...juneProps} now={new Date(2026, 6, 1, 0, 0, 1)} />,
    );

    expect(recovered).toHaveLength(0);
    expect(afterHydrate).toMatch(/Total (month to date|for the period)/);
    expect(afterHydrate).not.toContain("Projected");
  });
});
