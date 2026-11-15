import type { TranslationEntry } from "@/i18n";

const deApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API-Schlüssel",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Maschinen-Zugangsdaten für Skripte und Agents. Gilt für den gesamten Workspace — wer Schlüssel verwalten kann, sieht hier jeden Schlüssel, nicht nur die eigenen.",
    description: "Settings API Keys section card description",
  },
  "apiKeys.cliGuide": {
    message: "Die CLI einrichten.",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.colName": {
    message: "Name",
    description: "API Keys table column header",
  },
  "apiKeys.colClientId": {
    message: "Client-ID",
    description:
      "API Keys table column header — OAuth2 client_id paired with the secret",
  },
  "apiKeys.colCreated": {
    message: "Erstellt",
    description: "API Keys table column header",
  },
  "apiKeys.colCreatedBy": {
    message: "Erstellt von",
    description: "API Keys table column header — who minted the key",
  },
  "apiKeys.colLastUsed": {
    message: "Zuletzt verwendet",
    description:
      "API Keys table column header — when a token for the key was last used",
  },
  "apiKeys.neverUsed": {
    message: "Nie",
    description: "API Keys last-used cell when the key has never been used",
  },
  "apiKeys.emptyTitle": {
    message: "Keine API-Schlüssel",
    description: "API Keys empty-state title",
  },
  "apiKeys.emptyBody": {
    message:
      "Erstelle einen Schlüssel, um ein Skript oder einen Agent zu authentifizieren.",
    description: "API Keys empty-state body",
  },
  "apiKeys.forbiddenTitle": {
    message: "Nicht autorisiert",
    description: "API Keys state when the caller lacks permission (403)",
  },
  "apiKeys.forbiddenBody": {
    message:
      "Du hast keine Berechtigung, die API-Schlüssel dieses Workspace zu verwalten.",
    description: "API Keys forbidden-state body",
  },
  "apiKeys.errorTitle": {
    message: "API-Schlüssel konnten nicht geladen werden",
    description: "API Keys generic error title",
  },
  "apiKeys.errorBody": {
    message: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
    description: "API Keys generic error body",
  },
  "apiKeys.create": {
    message: "API-Schlüssel erstellen",
    description: "Button that opens the mint dialog",
  },
  "apiKeys.createTitle": {
    message: "API-Schlüssel erstellen",
    description: "Mint dialog title (name step)",
  },
  "apiKeys.createDescription": {
    message:
      "Gib diesem Schlüssel einen Namen, damit du ihn später wiedererkennst.",
    description: "Mint dialog description (name step)",
  },
  "apiKeys.fieldName": {
    message: "Name",
    description: "Mint dialog name field label",
  },
  "apiKeys.fieldNamePlaceholder": {
    message: "z. B. deploy-agent",
    description: "Mint dialog name field placeholder",
  },
  "apiKeys.createCancel": {
    message: "Abbrechen",
    description: "Mint dialog cancel button (name step)",
  },
  "apiKeys.createSubmit": {
    message: "Erstellen",
    description: "Mint dialog submit button (name step)",
  },
  "apiKeys.createdTitle": {
    message: "API-Schlüssel erstellt",
    description: "Mint dialog title (secret-shown step)",
  },
  "apiKeys.createdWarning": {
    message:
      "Kopiere jetzt die Client-ID und das Secret — das Secret kannst du danach nicht mehr einsehen.",
    description: "Mint dialog warning (secret-shown step)",
  },
  "apiKeys.fieldClientId": {
    message: "Client-ID",
    description: "Mint dialog label for the OAuth2 client_id",
  },
  "apiKeys.fieldSecret": {
    message: "Client-Secret",
    description: "Mint dialog label for the one-time client_secret",
  },
  "apiKeys.copyClientId": {
    message: "Client-ID kopieren",
    description: "Copy button aria-label for the client_id",
  },
  "apiKeys.copyClientIdNamed": {
    message: "Client-ID für {name} kopieren",
    description: "Copy button aria-label for one API client row's client_id",
  },
  "apiKeys.copySecret": {
    message: "Client-Secret kopieren",
    description: "Copy button aria-label for the client_secret",
  },
  "apiKeys.exchangeHelp": {
    message:
      "Dies ist ein OAuth2-client_credentials-Client, kein Bearer-Token. Tausche ihn unter {tokenEndpoint} ein:\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nSende keinen Scope: Der Schlüssel handelt mit voller Berechtigung in diesem Workspace, einschließlich Verbindungs-Strings, behandle ihn also wie ein Passwort. Verwende das zurückgegebene access_token als Authorization: Bearer.",
    description:
      "Mint dialog usage copy for the client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "Fertig",
    description: "Mint dialog close button (secret-shown step)",
  },
  "apiKeys.copy": {
    message: "Secret kopieren",
    description: "Copy-to-clipboard icon button label",
  },
  "apiKeys.copied": {
    message: "In die Zwischenablage kopiert",
    description: "Toast on a successful secret copy",
  },
  "apiKeys.copyError": {
    message: "Kopieren in die Zwischenablage fehlgeschlagen",
    description: "Toast on a failed secret copy",
  },
  "apiKeys.createSuccess": {
    message: "{name} erstellt",
    description: "Toast on a successful mint",
  },
  "apiKeys.createError": {
    message: "{name} konnte nicht erstellt werden",
    description: "Toast on a failed mint",
  },
  "apiKeys.revoke": {
    message: "Widerrufen",
    description: "Row action / confirmation button to revoke a key",
  },
  "apiKeys.revokeNamed": {
    message: "API-Schlüssel {name} widerrufen",
    description: "Row revoke button aria-label identifying the API key",
  },
  "apiKeys.revokeConfirmTitle": {
    message: "{name} widerrufen?",
    description: "Revoke-confirmation dialog title",
  },
  "apiKeys.revokeConfirmBody": {
    message:
      "Alles, was sich mit diesem Schlüssel authentifiziert, funktioniert sofort nicht mehr. Das lässt sich nicht rückgängig machen.",
    description: "Revoke-confirmation dialog body",
  },
  "apiKeys.revokeCancel": {
    message: "Abbrechen",
    description: "Revoke-confirmation dialog cancel button",
  },
  "apiKeys.revokeSuccess": {
    message: "{name} widerrufen",
    description: "Toast on a successful revoke",
  },
  "apiKeys.revokeError": {
    message: "{name} konnte nicht widerrufen werden",
    description: "Toast on a failed revoke",
  },
};

export default deApiKeys;
