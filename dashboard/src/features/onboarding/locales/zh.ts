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
      "不想添加银行卡？bex 是开源项目，您可以在自己的基础设施上运行，免费且无限制。",
    description:
      "Lead-in to the self-host exit on the payment wall (ADR075 § Positioning)",
  },
  "onboarding.paymentSetupSelfHost": {
    message: "删除账号并改为自托管",
    description:
      "Opens the confirmation dialog for the self-host exit on the payment wall",
  },
  "onboarding.paymentSetupSelfHostConfirmTitle": {
    message: "删除账号并改为自托管？",
    description: "Title of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmBody": {
    message:
      "这将永久删除您的 bex 账号和 workspace。我们从未向您收费，也不会保留任何数据。随后会带您前往自托管指南——bex 在您自己的基础设施上免费运行，没有任何限制。",
    description: "Body of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmAction": {
    message: "删除账号并继续",
    description: "Destructive confirm button in the self-host exit dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmCancel": {
    message: "保留我的账号",
    description: "Dismisses the self-host exit dialog without deleting anything",
  },
  "onboarding.paymentSetupSelfHostError": {
    message: "账号删除失败，未做任何更改。",
    description:
      "Shown when the self-host exit's deletion fails; the wall stays put rather than forwarding",
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
