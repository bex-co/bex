import type { TranslationEntry } from "@/i18n/config";

const zh: Record<string, TranslationEntry> = {
  "onboarding.paymentSetupTitle": {
    message: "添加付款方式",
    description: "Sign-up payment wall hero title (/setup/payment)",
  },
  "onboarding.paymentSetupSubtitle": {
    message: "完成这最后一步，您的工作区即可开始运行资源",
    description: "Sign-up payment wall hero subtitle",
  },
  "onboarding.paymentSetupCardTitle": {
    message: "需要付款方式",
    description: "Sign-up payment wall card title",
  },
  "onboarding.paymentSetupBody": {
    message:
      "托管版 bex 是付费产品：此工作区在创建或运行任何资源（包括免费层资源）之前，必须先登记一种付款方式。您只需为实际用量付费。",
    description:
      "Sign-up payment wall explanation (ADR075 D7: card required for all usage, free tier included)",
  },
  "onboarding.paymentSetupWorkspace": {
    message: "工作区：{name}",
    description: "Names the workspace the payment method will be bound to",
  },
  "onboarding.paymentSetupConfirming": {
    message: "已收到付款方式，正在向 Stripe 确认…",
    description:
      "Status after returning from Stripe Checkout while the webhook commit is awaited",
  },
  "onboarding.paymentSetupCancelled": {
    message: "已取消结账，未添加任何付款方式。",
    description: "Notice after returning from a cancelled Stripe Checkout",
  },
  "onboarding.paymentSetupSelfHostHint": {
    message:
      "不想添加银行卡？bex 是开源项目，您可以免费在自己的基础设施上运行。",
    description:
      "Lead-in to the self-host exit on the payment wall (ADR075 § Positioning)",
  },
  "onboarding.paymentSetupSelfHost": {
    message: "改为自托管 bex",
    description: "Link to the GitHub repository from the payment wall",
  },
  "onboarding.paymentSetupSignOut": {
    message: "退出登录",
    description: "Sign-out link on the payment wall",
  },
  "onboarding.paymentSetupRetry": {
    message: "重试",
    description: "Retries the billing readiness read after it failed",
  },
  "onboarding.paymentSetupContinue": {
    message: "继续前往控制台",
    description:
      "Escape hatch when billing readiness cannot be read; the API's own gate still applies",
  },
  "onboarding.paymentSetupContinuing": {
    message: "正在继续…",
    description:
      "Screen-reader status while the wall forwards a workspace that needs no payment step",
  },
};

export default zh;
