import type { TranslationEntry } from "@/i18n";

const deInvites: Record<string, TranslationEntry> = {
  "invites.title": {
    message: "Workspace-Einladung",
    description: "Invitation review: title",
  },
  "invites.authenticate": {
    message:
      "Erstelle einen Account oder melde dich an, um deine Workspace-Einladung zu prüfen.",
    description: "Invitation review: authenticate",
  },
  "invites.signUp": {
    message: "Registrieren",
    description: "Invitation review: signUp",
  },
  "invites.signIn": {
    message: "Anmelden",
    description: "Invitation review: signIn",
  },
  "invites.joinTitle": {
    message: "{workspace} beitreten",
    description: "Invitation review: joinTitle",
  },
  "invites.memberTitle": {
    message: "Du bist Mitglied von {workspace}",
    description: "Invitation review: memberTitle",
  },
  "invites.role": {
    message: "Du trittst als {role} bei.",
    description: "Invitation review: role",
  },
  "invites.memberRole": {
    message: "Deine Rolle ist {role}.",
    description: "Invitation review: memberRole",
  },
  "invites.inviter": {
    message: "Eingeladen von {email}",
    description: "Invitation review: inviter",
  },
  "invites.account": {
    message: "Angemeldet als {email}",
    description: "Invitation review: account",
  },
  "invites.join": {
    message: "Workspace beitreten",
    description: "Invitation review: join",
  },
  "invites.joining": {
    message: "Beitritt läuft…",
    description: "Invitation review: joining",
  },
  "invites.open": {
    message: "Workspace öffnen",
    description: "Invitation review: open",
  },
  "invites.opening": {
    message: "Wird geöffnet…",
    description: "Invitation review: opening",
  },
  "invites.notNow": {
    message: "Nicht jetzt",
    description: "Invitation review: notNow",
  },
  "invites.continue": {
    message: "Weiter zu bex",
    description: "Invitation review: continue",
  },
  "invites.unavailableTitle": {
    message: "Einladung nicht verfügbar",
    description: "Invitation review: unavailableTitle",
  },
  "invites.invalid": {
    message:
      "Dieser Einladungslink ist ungültig oder wurde widerrufen. Bitte den Workspace-Admin um eine neue Einladung.",
    description: "Invitation review: invalid",
  },
  "invites.expired": {
    message:
      "Diese Einladung ist abgelaufen. Bitte den Workspace-Admin, sie erneut zu senden.",
    description: "Invitation review: expired",
  },
  "invites.used": {
    message:
      "Diese Einladung wurde bereits verwendet. Melde dich mit dem Account an, der beigetreten ist, oder bitte um eine neue Einladung.",
    description: "Invitation review: used",
  },
  "invites.planLimit": {
    message:
      "Dieser Workspace kann mit seinem aktuellen Tarif kein weiteres Mitglied aufnehmen. Bitte den Workspace-Admin, Tarif und Plätze zu prüfen, und versuche es dann erneut.",
    description: "Invitation review: planLimit",
  },
  "invites.retryError": {
    message:
      "Wir konnten diese Anfrage nicht abschließen. Deine Einladung ist in diesem Tab gespeichert. Bitte versuche es erneut.",
    description: "Invitation review: retryError",
  },
  "invites.accessPending": {
    message:
      "Deine Mitgliedschaft ist bestätigt, aber der Workspace-Zugriff ist noch nicht bereit. Versuche, den Workspace erneut zu öffnen.",
    description: "Invitation review: accessPending",
  },
  "invites.storageUnavailable": {
    message:
      "Wir konnten deine Einladung in diesem Browser nicht speichern. Erlaube Website-Speicher und öffne den Einladungslink dann erneut.",
    description: "Invitation review: storageUnavailable",
  },
  "invites.retry": {
    message: "Erneut versuchen",
    description: "Invitation review: retry",
  },
};

export default deInvites;
