import type { TranslationEntry } from "@/i18n";

const pt: Record<string, TranslationEntry> = {
  "onboarding.paymentSetupTitle": {
    message: "Adicione uma forma de pagamento",
    description: "Sign-up payment wall hero title (/setup/payment)",
  },
  "onboarding.paymentSetupSubtitle": {
    message:
      "Falta só um passo para o seu workspace poder executar qualquer coisa",
    description: "Sign-up payment wall hero subtitle",
  },
  "onboarding.paymentSetupCardTitle": {
    message: "Forma de pagamento obrigatória",
    description: "Sign-up payment wall card title",
  },
  "onboarding.paymentSetupBody": {
    message:
      "O bex hospedado é um produto pago: é preciso ter uma forma de pagamento cadastrada para que este workspace possa criar ou executar qualquer recurso, inclusive os do plano gratuito. Você só paga pelo que usar.",
    description:
      "Sign-up payment wall explanation (ADR075 D7: card required for all usage, free tier included)",
  },
  "onboarding.paymentSetupWorkspace": {
    message: "Workspace: {name}",
    description: "Names the workspace the payment method will be bound to",
  },
  "onboarding.paymentSetupConfirming": {
    message: "Forma de pagamento recebida — confirmando com o Stripe…",
    description:
      "Status after returning from Stripe Checkout while the webhook commit is awaited",
  },
  "onboarding.paymentSetupCancelled": {
    message:
      "O checkout foi cancelado. Nenhuma forma de pagamento foi adicionada.",
    description: "Notice after returning from a canceled Stripe Checkout",
  },
  "onboarding.paymentSetupSelfHostHint": {
    message:
      "Prefere não adicionar um cartão? O bex é open source — execute-o na sua própria infraestrutura, de graça e sem limites.",
    description:
      "Lead-in to the self-host exit on the payment wall (ADR075 § Positioning)",
  },
  "onboarding.paymentSetupSelfHost": {
    message: "Excluir esta conta e fazer self-host",
    description:
      "Opens the confirmation dialog for the self-host exit on the payment wall",
  },
  "onboarding.paymentSetupSelfHostConfirmTitle": {
    message: "Excluir esta conta e fazer self-host?",
    description: "Title of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmBody": {
    message:
      "Isso exclui permanentemente sua conta e seu workspace do bex. Você nunca foi cobrado e nada é mantido. Vamos levar você ao guia de self-hosting — o bex roda de graça na sua própria infraestrutura, sem limites.",
    description: "Body of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmAction": {
    message: "Excluir conta e continuar",
    description: "Destructive confirm button in the self-host exit dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmCancel": {
    message: "Manter minha conta",
    description:
      "Dismisses the self-host exit dialog without deleting anything",
  },
  "onboarding.paymentSetupSelfHostError": {
    message: "Não foi possível excluir a conta. Nada foi alterado.",
    description:
      "Shown when the self-host exit's deletion fails; the wall stays put rather than forwarding",
  },
  "onboarding.paymentSetupSignOut": {
    message: "Sair",
    description: "Sign-out link on the payment wall",
  },
  "onboarding.paymentSetupRetry": {
    message: "Tentar novamente",
    description: "Retries the billing readiness read after it failed",
  },
  "onboarding.paymentSetupContinue": {
    message: "Continuar para o painel",
    description:
      "Escape hatch when billing readiness cannot be read; the API's own gate still applies",
  },
  "onboarding.paymentSetupContinuing": {
    message: "Continuando…",
    description:
      "Screen-reader status while the wall forwards a workspace that needs no payment step",
  },
  "onboarding.gettingStartedTitle": {
    message: "Comece a usar o bex",
    description:
      "Getting-started checklist title on an empty workspace Overview",
  },
  "onboarding.gettingStartedSubtitle": {
    message: "Alguns passos até o seu primeiro deploy.",
    description: "Getting-started checklist subtitle",
  },
  "onboarding.gettingStartedDismiss": {
    message: "Dispensar o guia de primeiros passos",
    description:
      "Accessible label of the getting-started checklist's dismiss button",
  },
  "onboarding.gettingStartedDone": {
    message: "Concluído",
    description: "Accessible label of a completed checklist item's icon",
  },
  "onboarding.gettingStartedPending": {
    message: "Ainda não concluído",
    description: "Accessible label of a pending checklist item's icon",
  },
  "onboarding.gettingStarted.createService": {
    message: "Crie seu primeiro serviço",
    description: "Checklist item linking to the new-service flow",
  },
  "onboarding.gettingStarted.connectGithub": {
    message: "Conecte o GitHub para fazer deploy a partir de um repositório",
    description: "Checklist item linking to Settings → Integrations",
  },
  "onboarding.gettingStarted.inviteTeammate": {
    message: "Convide um colega de equipe",
    description: "Checklist item linking to workspace settings (team)",
  },
  "onboarding.gettingStarted.upgradeToInvite": {
    message: "Faça upgrade do seu plano para convidar colegas de equipe",
    description:
      "Checklist item shown instead of Invite a teammate while every seat is used (Hobby); opens the plan change",
  },
};

export default pt;
