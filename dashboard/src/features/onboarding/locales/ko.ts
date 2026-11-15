import type { TranslationEntry } from "@/i18n";

const ko: Record<string, TranslationEntry> = {
  "onboarding.paymentSetupTitle": {
    message: "결제 수단 추가",
    description: "Sign-up payment wall hero title (/setup/payment)",
  },
  "onboarding.paymentSetupSubtitle": {
    message: "워크스페이스에서 무언가를 실행하기 전 마지막 단계입니다",
    description: "Sign-up payment wall hero subtitle",
  },
  "onboarding.paymentSetupCardTitle": {
    message: "결제 수단 필요",
    description: "Sign-up payment wall card title",
  },
  "onboarding.paymentSetupBody": {
    message:
      "호스팅형 bex는 유료 제품입니다. 무료 티어를 포함해 이 워크스페이스에서 리소스를 만들거나 실행하려면 먼저 결제 수단이 등록되어 있어야 합니다. 사용한 만큼만 청구됩니다.",
    description:
      "Sign-up payment wall explanation (ADR075 D7: card required for all usage, free tier included)",
  },
  "onboarding.paymentSetupWorkspace": {
    message: "워크스페이스: {name}",
    description: "Names the workspace the payment method will be bound to",
  },
  "onboarding.paymentSetupConfirming": {
    message: "결제 수단을 받았습니다 — Stripe에서 확인하는 중…",
    description:
      "Status after returning from Stripe Checkout while the webhook commit is awaited",
  },
  "onboarding.paymentSetupCancelled": {
    message: "체크아웃이 취소되었습니다. 결제 수단이 추가되지 않았습니다.",
    description: "Notice after returning from a canceled Stripe Checkout",
  },
  "onboarding.paymentSetupSelfHostHint": {
    message:
      "카드를 등록하고 싶지 않으신가요? bex는 오픈 소스입니다. 직접 운영하는 인프라에서 무료로, 제한 없이 실행할 수 있습니다.",
    description:
      "Lead-in to the self-host exit on the payment wall (ADR075 § Positioning)",
  },
  "onboarding.paymentSetupSelfHost": {
    message: "이 계정을 삭제하고 직접 호스팅",
    description:
      "Opens the confirmation dialog for the self-host exit on the payment wall",
  },
  "onboarding.paymentSetupSelfHostConfirmTitle": {
    message: "이 계정을 삭제하고 직접 호스팅하시겠습니까?",
    description: "Title of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmBody": {
    message:
      "bex 계정과 워크스페이스가 영구적으로 삭제됩니다. 요금이 청구된 적이 없으며 아무것도 보관되지 않습니다. 셀프 호스팅 가이드로 이동합니다. bex는 직접 운영하는 인프라에서 제한 없이 무료로 실행할 수 있습니다.",
    description: "Body of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmAction": {
    message: "계정 삭제 후 계속",
    description: "Destructive confirm button in the self-host exit dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmCancel": {
    message: "계정 유지",
    description:
      "Dismisses the self-host exit dialog without deleting anything",
  },
  "onboarding.paymentSetupSelfHostError": {
    message: "계정을 삭제하지 못했습니다. 변경된 내용은 없습니다.",
    description:
      "Shown when the self-host exit's deletion fails; the wall stays put rather than forwarding",
  },
  "onboarding.paymentSetupSignOut": {
    message: "로그아웃",
    description: "Sign-out link on the payment wall",
  },
  "onboarding.paymentSetupRetry": {
    message: "다시 시도",
    description: "Retries the billing readiness read after it failed",
  },
  "onboarding.paymentSetupContinue": {
    message: "대시보드로 계속",
    description:
      "Escape hatch when billing readiness cannot be read; the API's own gate still applies",
  },
  "onboarding.paymentSetupContinuing": {
    message: "계속하는 중…",
    description:
      "Screen-reader status while the wall forwards a workspace that needs no payment step",
  },
  "onboarding.gettingStartedTitle": {
    message: "bex 시작하기",
    description:
      "Getting-started checklist title on an empty workspace Overview",
  },
  "onboarding.gettingStartedSubtitle": {
    message: "첫 배포까지 몇 단계만 거치면 됩니다.",
    description: "Getting-started checklist subtitle",
  },
  "onboarding.gettingStartedDismiss": {
    message: "시작하기 닫기",
    description:
      "Accessible label of the getting-started checklist's dismiss button",
  },
  "onboarding.gettingStartedDone": {
    message: "완료",
    description: "Accessible label of a completed checklist item's icon",
  },
  "onboarding.gettingStartedPending": {
    message: "아직 완료되지 않음",
    description: "Accessible label of a pending checklist item's icon",
  },
  "onboarding.gettingStarted.createService": {
    message: "첫 서비스 만들기",
    description: "Checklist item linking to the new-service flow",
  },
  "onboarding.gettingStarted.connectGithub": {
    message: "GitHub를 연결하여 저장소에서 배포",
    description: "Checklist item linking to Settings → Integrations",
  },
  "onboarding.gettingStarted.inviteTeammate": {
    message: "팀원 초대",
    description: "Checklist item linking to workspace settings (team)",
  },
  "onboarding.gettingStarted.upgradeToInvite": {
    message: "요금제를 업그레이드하여 팀원 초대",
    description:
      "Checklist item shown instead of Invite a teammate while every seat is used (Hobby); opens the plan change",
  },
};

export default ko;
