import { afterEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import i18n from "@/i18n/init";
import { PaymentMethodRejectionAlert } from "../payment-method-rejection-alert";
import { BalanceDueAlert } from "@/common/components/balance-due-alert";

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("PaymentMethodRejectionAlert (ADR091 D10.5–D10.6)", () => {
  it("explains a refused prepaid card", () => {
    render(<PaymentMethodRejectionAlert reason="PREPAID_CARD" />);
    expect(
      screen.getByText(
        "Prepaid cards aren't accepted; add a credit or debit card.",
      ),
    ).toBeInTheDocument();
  });

  it("explains a card linked to an unpaid balance, from readiness's enum or an error's reason", () => {
    render(
      <>
        <PaymentMethodRejectionAlert reason="UNPAID_BALANCE" />
        <PaymentMethodRejectionAlert reason="unpaid_balance" />
      </>,
    );
    expect(
      screen.getAllByText(
        "This card is linked to an unpaid balance; add a different card.",
      ),
    ).toHaveLength(2);
  });

  it("renders nothing with no or an unknown reason", () => {
    const { container } = render(
      <>
        <PaymentMethodRejectionAlert reason="" />
        <PaymentMethodRejectionAlert reason="something_new" />
      </>,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("is translated into Chinese, as is the balance-due alert", async () => {
    await i18n.changeLanguage("zh");
    render(
      <>
        <PaymentMethodRejectionAlert reason="prepaid_card" />
        <BalanceDueAlert
          due={{
            workspaceId: "tea-1",
            amountDueCents: 300,
            currency: "usd",
            hostedInvoiceUrl: "https://invoice.stripe.com/i/in_1",
          }}
        />
      </>,
    );
    expect(
      screen.getByText("不接受预付卡；请添加信用卡或借记卡。"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "此工作区有 $3.00 的未付余额。请先支付账单，然后重新删除。",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /支付账单/ })).toHaveAttribute(
      "target",
      "_blank",
    );
  });
});
