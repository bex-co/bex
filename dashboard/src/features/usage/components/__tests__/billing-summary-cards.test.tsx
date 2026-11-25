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

import { afterEach, describe, expect, it } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n/init";
import {
  CreditBalanceCard,
  InvoiceHistoryCard,
} from "@/features/usage/components/billing-summary-cards";
import type { Billing, BillingInvoice } from "@/features/usage/hooks/use-usage";

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
          status: "PAID",
          amountUsd: "1234.50",
          creditsAppliedUsd: "0.00",
          amountDueUsd: "1234.50",
          currency: "USD",
          periodStart: "2026-07-01T00:00:00Z",
          periodEnd: "2026-08-01T00:00:00Z",
          hostedInvoiceUrl: null,
          invoicePdfUrl: null,
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
              status: "PAID",
              amountUsd: "74.78",
              creditsAppliedUsd: "74.78",
              amountDueUsd: "0.00",
              currency: "USD",
              periodStart: "2026-07-01T00:00:00Z",
              periodEnd: "2026-08-01T00:00:00Z",
              hostedInvoiceUrl: null,
              invoicePdfUrl: null,
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
      hostedInvoiceUrl: null,
      invoicePdfUrl: null,
    };
    render(
      <InvoiceHistoryCard
        billing={creditedPeriod({
          invoices: [
            {
              ...row,
              id: "in_charge",
              status: "PAID",
              kind: "agent_model_charge",
              amountUsd: "10.00",
              amountDueUsd: "10.00",
              periodStart: "2026-11-13T00:00:00Z",
            },
            {
              ...row,
              id: "in_cycle",
              status: "PAID",
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
              status: "PAID",
              kind: "cycle",
              amountUsd: "40.00",
              creditsAppliedUsd: "0.00",
              amountDueUsd: "40.00",
              currency: "USD",
              periodStart: "2026-10-01T00:00:00Z",
              periodEnd: "2026-11-01T00:00:00Z",
              hostedInvoiceUrl: null,
              invoicePdfUrl: null,
            },
          ],
        })}
      />,
    );

    expect(screen.queryByText("Agent model charge")).not.toBeInTheDocument();
    expect(screen.queryByText(/deducted/)).not.toBeInTheDocument();
  });
});

// w6/m625: each invoice's row menu offers Stripe's PDF, and Pay Now (Stripe's
// hosted invoice page) while the invoice still owes, as Render's Billing page
// does; statuses read in the dashboard's language, not Stripe's raw words.
describe("InvoiceHistoryCard row menu (w6/m625)", () => {
  const PDF = "https://pay.stripe.com/invoice/acct_1/test_1/pdf?s=ap";
  const HOSTED = "https://invoice.stripe.com/i/acct_1/test_1";

  function invoice(over: Partial<BillingInvoice>): BillingInvoice {
    return {
      id: "in_1",
      kind: "cycle",
      status: "PAID",
      amountUsd: "40.00",
      creditsAppliedUsd: "0.00",
      amountDueUsd: "40.00",
      currency: "USD",
      periodStart: "2026-10-01T00:00:00Z",
      periodEnd: "2026-11-01T00:00:00Z",
      hostedInvoiceUrl: HOSTED,
      invoicePdfUrl: PDF,
      ...over,
    };
  }

  function renderInvoices(invoices: BillingInvoice[]) {
    return render(
      <InvoiceHistoryCard billing={creditedPeriod({ invoices })} />,
    );
  }

  /** The items of the row menu for `period`, opened as a user opens it. */
  async function menuItems(period: string) {
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("button", {
        name: `Actions for the ${period} invoice`,
      }),
    );
    return within(await screen.findByRole("menu")).getAllByRole("menuitem");
  }

  afterEach(async () => {
    await act(async () => {
      await i18n.changeLanguage("en");
    });
  });

  it("offers a paid invoice its PDF only", async () => {
    renderInvoices([invoice({ status: "PAID" })]);
    const items = await menuItems("October 2026");
    expect(items.map((item) => item.textContent)).toEqual(["Download PDF"]);
    expect(items[0]).toHaveAttribute("href", PDF);
    expect(items[0]).toHaveAttribute("target", "_blank");
    expect(items[0]).toHaveAttribute("rel", "noopener noreferrer");
  });

  it.each(["OPEN", "UNCOLLECTIBLE"])(
    "offers an %s invoice Pay Now on Stripe's hosted page, then its PDF",
    async (status) => {
      renderInvoices([invoice({ status })]);
      const items = await menuItems("October 2026");
      expect(items.map((item) => item.textContent)).toEqual([
        "Pay Now",
        "Download PDF",
      ]);
      expect(items[0]).toHaveAttribute("href", HOSTED);
      expect(items[0]).toHaveAttribute("target", "_blank");
      expect(items[0]).toHaveAttribute("rel", "noopener noreferrer");
    },
  );

  it("offers no Pay Now on a void invoice, which owes nothing", async () => {
    renderInvoices([invoice({ status: "VOID" })]);
    const items = await menuItems("October 2026");
    expect(items.map((item) => item.textContent)).toEqual(["Download PDF"]);
  });

  it("offers Pay Now alone when Stripe gave an open invoice no PDF", async () => {
    renderInvoices([invoice({ status: "OPEN", invoicePdfUrl: null })]);
    const items = await menuItems("October 2026");
    expect(items.map((item) => item.textContent)).toEqual(["Pay Now"]);
  });

  it("draws no menu for an invoice Stripe gave no link", () => {
    renderInvoices([
      invoice({ status: "OPEN", hostedInvoiceUrl: null, invoicePdfUrl: null }),
    ]);
    expect(screen.getByText("$40.00")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("refuses a link that is not http(s)", () => {
    renderInvoices([
      invoice({
        status: "OPEN",
        hostedInvoiceUrl: "javascript:alert(1)",
        invoicePdfUrl: "data:text/html,hi",
      }),
    ]);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("gives each row its own menu", async () => {
    renderInvoices([
      invoice({
        id: "in_open",
        status: "OPEN",
        periodStart: "2026-10-01T00:00:00Z",
      }),
      invoice({
        id: "in_paid",
        status: "PAID",
        periodStart: "2026-09-01T00:00:00Z",
      }),
    ]);
    const items = await menuItems("September 2026");
    expect(items.map((item) => item.textContent)).toEqual(["Download PDF"]);
  });

  it("translates each status instead of showing Stripe's word", async () => {
    renderInvoices([
      invoice({ id: "a", status: "OPEN" }),
      invoice({ id: "b", status: "PAID" }),
      invoice({ id: "c", status: "VOID" }),
      invoice({ id: "d", status: "UNCOLLECTIBLE" }),
    ]);
    for (const label of ["Unpaid", "Paid", "Void", "Uncollectible"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    for (const raw of ["OPEN", "PAID", "VOID", "UNCOLLECTIBLE"]) {
      expect(screen.queryByText(raw)).not.toBeInTheDocument();
    }

    await act(async () => {
      await i18n.changeLanguage("zh");
    });
    for (const label of ["未支付", "已支付", "已作废", "无法收回"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });
});
