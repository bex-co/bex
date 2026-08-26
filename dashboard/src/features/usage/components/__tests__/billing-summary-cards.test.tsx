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
    expect(screen.getByText("$1000.00")).toBeInTheDocument();
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

describe("InvoiceHistoryCard", () => {
  it("shows what a credited invoice collected, not its gross charge", () => {
    render(
      <InvoiceHistoryCard
        billing={creditedPeriod({
          invoices: [
            {
              id: "in_1",
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
});
