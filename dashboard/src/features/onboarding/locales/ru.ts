import type { TranslationEntry } from "@/i18n";

const ru: Record<string, TranslationEntry> = {
  "onboarding.paymentSetupTitle": {
    message: "Добавьте способ оплаты",
    description: "Sign-up payment wall hero title (/setup/payment)",
  },
  "onboarding.paymentSetupSubtitle": {
    message:
      "Остался последний шаг, чтобы ваше рабочее пространство могло что-либо запускать",
    description: "Sign-up payment wall hero subtitle",
  },
  "onboarding.paymentSetupCardTitle": {
    message: "Нужен способ оплаты",
    description: "Sign-up payment wall card title",
  },
  "onboarding.paymentSetupBody": {
    message:
      "Облачный bex — платный продукт: чтобы это рабочее пространство могло создавать или запускать любые ресурсы, включая бесплатные, должен быть указан способ оплаты. Вы платите только за то, что используете.",
    description:
      "Sign-up payment wall explanation (ADR075 D7: card required for all usage, free tier included)",
  },
  "onboarding.paymentSetupWorkspace": {
    message: "Рабочее пространство: {name}",
    description: "Names the workspace the payment method will be bound to",
  },
  "onboarding.paymentSetupConfirming": {
    message: "Способ оплаты получен — подтверждаем через Stripe…",
    description:
      "Status after returning from Stripe Checkout while the webhook commit is awaited",
  },
  "onboarding.paymentSetupCancelled": {
    message: "Оформление отменено. Способ оплаты не добавлен.",
    description: "Notice after returning from a canceled Stripe Checkout",
  },
  "onboarding.paymentSetupSelfHostHint": {
    message:
      "Не хотите добавлять карту? bex — проект с открытым исходным кодом: запустите его на своей инфраструктуре бесплатно и без ограничений.",
    description:
      "Lead-in to the self-host exit on the payment wall (ADR075 § Positioning)",
  },
  "onboarding.paymentSetupSelfHost": {
    message: "Удалить этот аккаунт и развернуть у себя",
    description:
      "Opens the confirmation dialog for the self-host exit on the payment wall",
  },
  "onboarding.paymentSetupSelfHostConfirmTitle": {
    message: "Удалить этот аккаунт и развернуть у себя?",
    description: "Title of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmBody": {
    message:
      "Это навсегда удалит ваш аккаунт и рабочее пространство bex. С вас ничего не списывалось, и ничего не сохраняется. Мы перенаправим вас к руководству по самостоятельному развёртыванию — bex бесплатно работает на вашей инфраструктуре без ограничений.",
    description: "Body of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmAction": {
    message: "Удалить аккаунт и продолжить",
    description: "Destructive confirm button in the self-host exit dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmCancel": {
    message: "Оставить мой аккаунт",
    description:
      "Dismisses the self-host exit dialog without deleting anything",
  },
  "onboarding.paymentSetupSelfHostError": {
    message: "Не удалось удалить аккаунт. Ничего не изменилось.",
    description:
      "Shown when the self-host exit's deletion fails; the wall stays put rather than forwarding",
  },
  "onboarding.paymentSetupSignOut": {
    message: "Выйти",
    description: "Sign-out link on the payment wall",
  },
  "onboarding.paymentSetupRetry": {
    message: "Повторить",
    description: "Retries the billing readiness read after it failed",
  },
  "onboarding.paymentSetupContinue": {
    message: "Перейти в панель",
    description:
      "Escape hatch when billing readiness cannot be read; the API's own gate still applies",
  },
  "onboarding.paymentSetupContinuing": {
    message: "Переходим…",
    description:
      "Screen-reader status while the wall forwards a workspace that needs no payment step",
  },
  "onboarding.gettingStartedTitle": {
    message: "Начало работы с bex",
    description:
      "Getting-started checklist title on an empty workspace Overview",
  },
  "onboarding.gettingStartedSubtitle": {
    message: "Несколько шагов до первого деплоя.",
    description: "Getting-started checklist subtitle",
  },
  "onboarding.gettingStartedDismiss": {
    message: "Скрыть начало работы",
    description:
      "Accessible label of the getting-started checklist's dismiss button",
  },
  "onboarding.gettingStartedDone": {
    message: "Готово",
    description: "Accessible label of a completed checklist item's icon",
  },
  "onboarding.gettingStartedPending": {
    message: "Ещё не выполнено",
    description: "Accessible label of a pending checklist item's icon",
  },
  "onboarding.gettingStarted.createService": {
    message: "Создайте первый сервис",
    description: "Checklist item linking to the new-service flow",
  },
  "onboarding.gettingStarted.connectGithub": {
    message: "Подключите GitHub, чтобы деплоить из репозитория",
    description: "Checklist item linking to Settings → Integrations",
  },
  "onboarding.gettingStarted.inviteTeammate": {
    message: "Пригласите коллегу",
    description: "Checklist item linking to workspace settings (team)",
  },
  "onboarding.gettingStarted.upgradeToInvite": {
    message: "Перейдите на другой тариф, чтобы приглашать коллег",
    description:
      "Checklist item shown instead of Invite a teammate while every seat is used (Hobby); opens the plan change",
  },
};

export default ru;
