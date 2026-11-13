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
import { render, screen } from "@testing-library/react";
import {
  CreditBalanceCard,
  InvoiceHistoryCard,
} from "@/features/usage/components/billing-summary-cards";
import type { Billing } from "@/features/usage/hooks/use-usage";

// The live w6/m98 shape: a $1,000 grant absorbed the whole $74.78 period, so
// Stripe's invoice collects nothing while the charge itself is real.
function creditedPeriod(over: Partial<Billing> = {}): Billing {
  return {
    currentCost: {
      amountUsd: "74.78",
      creditsAppliedUsd: "74.78",
      amountDueUsd: "0.00",
      currency: "USD",
      periodStart: "2026-08-01T00:00:00Z",
      periodEnd: "2026-09-01T00:00:00Z",
    },
    invoices: [],
    credits: {
      availableUsd: "1000.00",
      currency: "USD",
      grants: [
        {
          name: "superadmin's privilege",
          remainingUsd: "1000.00",
          expiresAt: "",
        },
      ],
    },
    ...over,
  };
}

describe("CreditBalanceCard", () => {
  it("reports the credit Stripe actually applied, not min(balance, cost)", () => {
    // The old client-side arithmetic read the netted-to-zero cost and derived
    // "$0.00 applied" for the very period a grant covered in full.
    render(<CreditBalanceCard billing={creditedPeriod()} />);

    expect(
      screen.getByText(/Credits applied −\$74\.78 → amount due \$0\.00/),
    ).toBeInTheDocument();
    expect(screen.getByText("$1,000.00")).toBeInTheDocument();
  });

  it("shows what is still owed when credit only partly covers the period", () => {
    render(
      <CreditBalanceCard
        billing={creditedPeriod({
          currentCost: {
            amountUsd: "74.78",
            creditsAppliedUsd: "10.00",
            amountDueUsd: "64.78",
            currency: "USD",
            periodStart: "2026-08-01T00:00:00Z",
            periodEnd: "2026-09-01T00:00:00Z",
          },
        })}
      />,
    );

    expect(
      screen.getByText(/Credits applied −\$10\.00 → amount due \$64\.78/),
    ).toBeInTheDocument();
  });

  it("renders nothing for a workspace that holds no credit", () => {
    const { container } = render(
      <CreditBalanceCard billing={creditedPeriod({ credits: null })} />,
    );

    expect(container).toBeEmptyDOMElement();
  });
});

// w21/024: every amount goes through the locale-aware money helper, so a value
// of 1,000 or more is grouped and the messages carry no literal `$` of their own.
describe("grouped money amounts", () => {
  it("groups the balance, the applied/due line, the expiry note and an invoice", () => {
    const base = creditedPeriod();
    const big = creditedPeriod({
      currentCost: {
        ...base.currentCost!,
        amountUsd: "1234.50",
        creditsAppliedUsd: "1234.50",
        amountDueUsd: "1234.50",
      },
      credits: {
        ...base.credits!,
        availableUsd: "1000551.64",
        grants: [
          {
            name: "grant",
            remainingUsd: "1234.50",
            expiresAt: "2026-12-31T00:00:00Z",
          },
        ],
      },
      invoices: [
        {
          id: "in_big",
          kind: "cycle",
          status: "paid",
          amountUsd: "1234.50",
          creditsAppliedUsd: "0.00",
          amountDueUsd: "1234.50",
          currency: "USD",
          periodStart: "2026-07-01T00:00:00Z",
          periodEnd: "2026-08-01T00:00:00Z",
        },
      ],
    });
    const { container } = render(
      <>
        <CreditBalanceCard billing={big} />
        <InvoiceHistoryCard billing={big} />
      </>,
    );
    expect(screen.getByText("$1,000,551.64")).toBeInTheDocument();
    expect(
      screen.getByText("Credits applied −$1,234.50 → amount due $1,234.50"),
    ).toBeInTheDocument();
    expect(screen.getByText(/^\$1,234\.50 of it expires/)).toBeInTheDocument();
    expect(container.textContent).not.toMatch(/\$\$/);
    expect(
      screen.getByText("$1,234.50", { selector: "td" }),
    ).toBeInTheDocument();
  });
});

describe("InvoiceHistoryCard", () => {
  it("shows what a credited invoice collected, not its gross charge", () => {
    render(
      <InvoiceHistoryCard
        billing={creditedPeriod({
          invoices: [
            {
              id: "in_1",
              kind: "cycle",
              status: "paid",
              amountUsd: "74.78",
              creditsAppliedUsd: "74.78",
              amountDueUsd: "0.00",
              currency: "USD",
              periodStart: "2026-07-01T00:00:00Z",
              periodEnd: "2026-08-01T00:00:00Z",
            },
          ],
        })}
      />,
    );

    expect(screen.getByText("$0.00")).toBeInTheDocument();
    expect(screen.queryByText("$74.78")).not.toBeInTheDocument();
  });

  it("labels an agent model charge and says the month's invoice deducts it", () => {
    const row = {
      creditsAppliedUsd: "0.00",
      currency: "USD",
      periodEnd: "2026-11-13T00:00:00Z",
    };
    render(
      <InvoiceHistoryCard
        billing={creditedPeriod({
          invoices: [
            {
              ...row,
              id: "in_charge",
              status: "paid",
              kind: "agent_model_charge",
              amountUsd: "10.00",
              amountDueUsd: "10.00",
              periodStart: "2026-11-13T00:00:00Z",
            },
            {
              ...row,
              id: "in_cycle",
              status: "paid",
              kind: "cycle",
              amountUsd: "40.00",
              amountDueUsd: "40.00",
              periodStart: "2026-10-01T00:00:00Z",
            },
          ],
        })}
      />,
    );

    expect(screen.getAllByText("Agent model charge")).toHaveLength(1);
    expect(
      screen.getByText(/Agent model charges made during a month are deducted/),
    ).toBeInTheDocument();
  });

  it("adds no charge note when the history has only cycle invoices", () => {
    render(
      <InvoiceHistoryCard
        billing={creditedPeriod({
          invoices: [
            {
              id: "in_cycle",
              status: "paid",
              kind: "cycle",
              amountUsd: "40.00",
              creditsAppliedUsd: "0.00",
              amountDueUsd: "40.00",
              currency: "USD",
              periodStart: "2026-10-01T00:00:00Z",
              periodEnd: "2026-11-01T00:00:00Z",
            },
          ],
        })}
      />,
    );

    expect(screen.queryByText("Agent model charge")).not.toBeInTheDocument();
    expect(screen.queryByText(/deducted/)).not.toBeInTheDocument();
  });
});
