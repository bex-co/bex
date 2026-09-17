import type { TranslationEntry } from "@/i18n/config";

const en: Record<string, TranslationEntry> = {
  "onboarding.paymentSetupTitle": {
    message: "Add a payment method",
    description: "Sign-up payment wall hero title (/setup/payment)",
  },
  "onboarding.paymentSetupSubtitle": {
    message: "One last step before your workspace can run anything",
    description: "Sign-up payment wall hero subtitle",
  },
  "onboarding.paymentSetupCardTitle": {
    message: "Payment method required",
    description: "Sign-up payment wall card title",
  },
  "onboarding.paymentSetupBody": {
    message:
      "Hosted bex is a paid product: a payment method must be on file before this workspace can create or run any resource, including free-tier ones. You are only charged for what you use.",
    description:
      "Sign-up payment wall explanation (ADR075 D7: card required for all usage, free tier included)",
  },
  "onboarding.paymentSetupWorkspace": {
    message: "Workspace: {name}",
    description: "Names the workspace the payment method will be bound to",
  },
  "onboarding.paymentSetupConfirming": {
    message: "Payment method received — confirming with Stripe…",
    description:
      "Status after returning from Stripe Checkout while the webhook commit is awaited",
  },
  "onboarding.paymentSetupCancelled": {
    message: "Checkout was cancelled. No payment method was added.",
    description: "Notice after returning from a cancelled Stripe Checkout",
  },
  "onboarding.paymentSetupSelfHostHint": {
    message:
      "Prefer not to add a card? bex is open source — run it on your own infrastructure, free and unlimited.",
    description:
      "Lead-in to the self-host exit on the payment wall (ADR075 § Positioning)",
  },
  "onboarding.paymentSetupSelfHost": {
    message: "Delete this account and self-host",
    description:
      "Opens the confirmation dialog for the self-host exit on the payment wall",
  },
  "onboarding.paymentSetupSelfHostConfirmTitle": {
    message: "Delete this account and self-host?",
    description: "Title of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmBody": {
    message:
      "This permanently deletes your bex account and workspace. You were never charged, and nothing is kept. We'll send you to the self-hosting guide — bex runs free on your own infrastructure, with no limits.",
    description: "Body of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmAction": {
    message: "Delete account and continue",
    description: "Destructive confirm button in the self-host exit dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmCancel": {
    message: "Keep my account",
    description: "Dismisses the self-host exit dialog without deleting anything",
  },
  "onboarding.paymentSetupSelfHostError": {
    message: "The account could not be deleted. Nothing was changed.",
    description:
      "Shown when the self-host exit's deletion fails; the wall stays put rather than forwarding",
  },
  "onboarding.paymentSetupSignOut": {
    message: "Sign out",
    description: "Sign-out link on the payment wall",
  },
  "onboarding.paymentSetupRetry": {
    message: "Try again",
    description: "Retries the billing readiness read after it failed",
  },
  "onboarding.paymentSetupContinue": {
    message: "Continue to the dashboard",
    description:
      "Escape hatch when billing readiness cannot be read; the API's own gate still applies",
  },
  "onboarding.paymentSetupContinuing": {
    message: "Continuing…",
    description:
      "Screen-reader status while the wall forwards a workspace that needs no payment step",
  },
};

export default en;
