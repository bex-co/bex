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

import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { ChargesCard } from "@/features/usage/components/charges-card";
import { groupByCategory, projectMonthEnd } from "@/features/usage/lib/charges";
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
