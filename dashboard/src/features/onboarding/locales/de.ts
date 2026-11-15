import type { TranslationEntry } from "@/i18n";

const de: Record<string, TranslationEntry> = {
  "onboarding.paymentSetupTitle": {
    message: "Zahlungsmethode hinzufügen",
    description: "Sign-up payment wall hero title (/setup/payment)",
  },
  "onboarding.paymentSetupSubtitle": {
    message:
      "Noch ein letzter Schritt, bevor dein Workspace etwas ausführen kann",
    description: "Sign-up payment wall hero subtitle",
  },
  "onboarding.paymentSetupCardTitle": {
    message: "Zahlungsmethode erforderlich",
    description: "Sign-up payment wall card title",
  },
  "onboarding.paymentSetupBody": {
    message:
      "Gehostetes bex ist ein kostenpflichtiges Produkt: Es muss eine Zahlungsmethode hinterlegt sein, bevor dieser Workspace Ressourcen erstellen oder ausführen kann, auch kostenlose. Dir wird nur berechnet, was du nutzt.",
    description:
      "Sign-up payment wall explanation (ADR075 D7: card required for all usage, free tier included)",
  },
  "onboarding.paymentSetupWorkspace": {
    message: "Workspace: {name}",
    description: "Names the workspace the payment method will be bound to",
  },
  "onboarding.paymentSetupConfirming": {
    message: "Zahlungsmethode erhalten — Bestätigung durch Stripe läuft…",
    description:
      "Status after returning from Stripe Checkout while the webhook commit is awaited",
  },
  "onboarding.paymentSetupCancelled": {
    message:
      "Der Checkout wurde abgebrochen. Es wurde keine Zahlungsmethode hinzugefügt.",
    description: "Notice after returning from a canceled Stripe Checkout",
  },
  "onboarding.paymentSetupSelfHostHint": {
    message:
      "Du möchtest keine Karte hinterlegen? bex ist Open Source — betreibe es auf deiner eigenen Infrastruktur, kostenlos und unbegrenzt.",
    description:
      "Lead-in to the self-host exit on the payment wall (ADR075 § Positioning)",
  },
  "onboarding.paymentSetupSelfHost": {
    message: "Dieses Konto löschen und selbst hosten",
    description:
      "Opens the confirmation dialog for the self-host exit on the payment wall",
  },
  "onboarding.paymentSetupSelfHostConfirmTitle": {
    message: "Dieses Konto löschen und selbst hosten?",
    description: "Title of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmBody": {
    message:
      "Dadurch werden dein bex-Konto und dein Workspace dauerhaft gelöscht. Dir wurde nie etwas berechnet und es bleibt nichts erhalten. Wir leiten dich zur Self-Hosting-Anleitung weiter — bex läuft kostenlos und ohne Limits auf deiner eigenen Infrastruktur.",
    description: "Body of the self-host exit confirmation dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmAction": {
    message: "Konto löschen und fortfahren",
    description: "Destructive confirm button in the self-host exit dialog",
  },
  "onboarding.paymentSetupSelfHostConfirmCancel": {
    message: "Mein Konto behalten",
    description:
      "Dismisses the self-host exit dialog without deleting anything",
  },
  "onboarding.paymentSetupSelfHostError": {
    message:
      "Das Konto konnte nicht gelöscht werden. Es wurde nichts geändert.",
    description:
      "Shown when the self-host exit's deletion fails; the wall stays put rather than forwarding",
  },
  "onboarding.paymentSetupSignOut": {
    message: "Abmelden",
    description: "Sign-out link on the payment wall",
  },
  "onboarding.paymentSetupRetry": {
    message: "Erneut versuchen",
    description: "Retries the billing readiness read after it failed",
  },
  "onboarding.paymentSetupContinue": {
    message: "Weiter zum Dashboard",
    description:
      "Escape hatch when billing readiness cannot be read; the API's own gate still applies",
  },
  "onboarding.paymentSetupContinuing": {
    message: "Weiter…",
    description:
      "Screen-reader status while the wall forwards a workspace that needs no payment step",
  },
  "onboarding.gettingStartedTitle": {
    message: "Erste Schritte mit bex",
    description:
      "Getting-started checklist title on an empty workspace Overview",
  },
  "onboarding.gettingStartedSubtitle": {
    message: "Ein paar Schritte bis zu deinem ersten Deployment.",
    description: "Getting-started checklist subtitle",
  },
  "onboarding.gettingStartedDismiss": {
    message: "Erste Schritte ausblenden",
    description:
      "Accessible label of the getting-started checklist's dismiss button",
  },
  "onboarding.gettingStartedDone": {
    message: "Erledigt",
    description: "Accessible label of a completed checklist item's icon",
  },
  "onboarding.gettingStartedPending": {
    message: "Noch nicht erledigt",
    description: "Accessible label of a pending checklist item's icon",
  },
  "onboarding.gettingStarted.createService": {
    message: "Erstelle deinen ersten Service",
    description: "Checklist item linking to the new-service flow",
  },
  "onboarding.gettingStarted.connectGithub": {
    message: "Verbinde GitHub, um aus einem Repository zu deployen",
    description: "Checklist item linking to Settings → Integrations",
  },
  "onboarding.gettingStarted.inviteTeammate": {
    message: "Lade ein Teammitglied ein",
    description: "Checklist item linking to workspace settings (team)",
  },
  "onboarding.gettingStarted.upgradeToInvite": {
    message: "Upgrade deinen Tarif, um Teammitglieder einzuladen",
    description:
      "Checklist item shown instead of Invite a teammate while every seat is used (Hobby); opens the plan change",
  },
};

export default de;
