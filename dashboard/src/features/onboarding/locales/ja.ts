import type { TranslationEntry } from "@/i18n";

const ja: Record<string, TranslationEntry> = {
  "onboarding.paymentSetupTitle": {
    message: "お支払い方法を追加",
    description: "Sign-up payment wall hero title (/setup/payment)",
  },
  "onboarding.paymentSetupSubtitle": {
    message: "ワークスペースを利用開始する前の最後のステップです",
    description: "Sign-up payment wall hero subtitle",
  },
  "onboarding.paymentSetupCardTitle": {
    message: "お支払い方法が必要です",
    description: "Sign-up payment wall card title",
  },
  "onboarding.paymentSetupBody": {
    message:
      "ホスト版 bex は有料のプロダクトです。無料枠を含め、このワークスペースでリソースを作成・実行するには、お支払い方法の登録が必要です。課金されるのは実際に使用した分のみです。",
    description:
      "Sign-up payment wall explanation (ADR075 D7: card required for all usage, free tier included)",
  },
  "onboarding.paymentSetupWorkspace": {
    message: "ワークスペース: {name}",
    description: "Names the workspace the payment method will be bound to",
  },
  "onboarding.paymentSetupConfirming": {
    message: "お支払い方法を受け取りました。Stripe で確認しています…",
    description:
      "Status after returning from Stripe Checkout while the webhook commit is awaited",
  },
  "onboarding.paymentSetupCancelled": {
    message:
      "チェックアウトがキャンセルされました。お支払い方法は追加されていません。",
    description: "Notice after returning from a canceled Stripe Checkout",
  },
  "onboarding.paymentSetupSelfHostHint": {
    message:
      "カードを登録したくない場合は、bex はオープンソースなので、ご自身のインフラで無料・無制限に実行できます。",
    description:
      "Lead-in to the self-host exit on the payment wall (ADR075 § Positioning)",
  },
  "onboarding.paymentSetupSelfHost": {
    message: "このアカウントを削除してセルフホストする",
    description:
      "Opens the confirmation dialog for the self-host exit on the payment wall",
  },
  "onboarding.paymentSetupSelfHostConfirmTitle": {
    message: "このアカウントを削除してセルフホストしますか？",
    description: "Title of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmBody": {
    message:
      "bex のアカウントとワークスペースが完全に削除されます。課金は一切発生しておらず、データは何も残りません。セルフホスティングのガイドへ移動します。bex はご自身のインフラで制限なく無料で実行できます。",
    description: "Body of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmAction": {
    message: "アカウントを削除して続行",
    description: "Destructive confirm button in the self-host exit dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmCancel": {
    message: "アカウントを残す",
    description:
      "Dismisses the self-host exit dialog without deleting anything",
  },
  "onboarding.paymentSetupSelfHostError": {
    message: "アカウントを削除できませんでした。何も変更されていません。",
    description:
      "Shown when the self-host exit's deletion fails; the wall stays put rather than forwarding",
  },
  "onboarding.paymentSetupSignOut": {
    message: "ログアウト",
    description: "Sign-out link on the payment wall",
  },
  "onboarding.paymentSetupRetry": {
    message: "再試行",
    description: "Retries the billing readiness read after it failed",
  },
  "onboarding.paymentSetupContinue": {
    message: "ダッシュボードに進む",
    description:
      "Escape hatch when billing readiness cannot be read; the API's own gate still applies",
  },
  "onboarding.paymentSetupContinuing": {
    message: "続行しています…",
    description:
      "Screen-reader status while the wall forwards a workspace that needs no payment step",
  },
  "onboarding.gettingStartedTitle": {
    message: "bex を始めましょう",
    description:
      "Getting-started checklist title on an empty workspace Overview",
  },
  "onboarding.gettingStartedSubtitle": {
    message: "最初のデプロイまであと数ステップです。",
    description: "Getting-started checklist subtitle",
  },
  "onboarding.gettingStartedDismiss": {
    message: "はじめにを閉じる",
    description:
      "Accessible label of the getting-started checklist's dismiss button",
  },
  "onboarding.gettingStartedDone": {
    message: "完了",
    description: "Accessible label of a completed checklist item's icon",
  },
  "onboarding.gettingStartedPending": {
    message: "未完了",
    description: "Accessible label of a pending checklist item's icon",
  },
  "onboarding.gettingStarted.createService": {
    message: "最初のサービスを作成",
    description: "Checklist item linking to the new-service flow",
  },
  "onboarding.gettingStarted.connectGithub": {
    message: "GitHub を連携してリポジトリからデプロイ",
    description: "Checklist item linking to Settings → Integrations",
  },
  "onboarding.gettingStarted.inviteTeammate": {
    message: "チームメイトを招待",
    description: "Checklist item linking to workspace settings (team)",
  },
  "onboarding.gettingStarted.upgradeToInvite": {
    message: "プランをアップグレードしてチームメイトを招待",
    description:
      "Checklist item shown instead of Invite a teammate while every seat is used (Hobby); opens the plan change",
  },
};

export default ja;
