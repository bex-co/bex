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
      "API Keys table column header: the key's id, which is also its OAuth2 client_id",
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
    description: "API Keys table column header: when the key was last used",
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
    description: "Mint dialog title (key-shown step)",
  },
  "apiKeys.createdWarning": {
    message:
      "Kopiere den Schlüssel jetzt. Du kannst ihn danach nicht mehr einsehen.",
    description:
      "Mint dialog warning (key-shown step): the key is shown only once",
  },
  "apiKeys.fieldKey": {
    message: "API-Schlüssel",
    description:
      "Mint dialog label for the key itself, the one string sent as Authorization: Bearer",
  },
  "apiKeys.copyKey": {
    message: "API-Schlüssel kopieren",
    description: "Copy button aria-label for the key",
  },
  "apiKeys.keyHelp": {
    message:
      "Sende ihn bei jeder Anfrage als Authorization: Bearer. Der Schlüssel handelt mit voller Berechtigung in diesem Workspace, einschließlich Verbindungs-Strings, behandle ihn also wie ein Passwort.",
    description:
      "Mint dialog copy under the key: how to send it, and that it carries full authority in the workspace",
  },
  "apiKeys.curlTitle": {
    message: "Beispielanfrage",
    description:
      "Mint dialog label for the curl example that lists services with the key",
  },
  "apiKeys.copyCurl": {
    message: "Beispielanfrage kopieren",
    description: "Copy button aria-label for the curl example",
  },
  "apiKeys.oauthDetails": {
    message: "OAuth-Client-Zugangsdaten",
    description:
      "Mint dialog title of the collapsed detail holding the client ID, the client secret and the optional token exchange",
  },
  "apiKeys.fieldClientId": {
    message: "Client-ID",
    description:
      "Mint dialog label for the OAuth2 client_id, inside the OAuth client credentials detail",
  },
  "apiKeys.fieldSecret": {
    message: "Client-Secret",
    description:
      "Mint dialog label for the one-time client_secret, inside the OAuth client credentials detail",
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
      "Dieser Schlüssel funktioniert auch als OAuth2-Client. Wenn du kurzlebige Tokens bevorzugst, tausche die Client-ID und das Client-Secret unter {tokenEndpoint} gegen ein Access-Token ein:\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nSende keinen Scope. Verwende das zurückgegebene access_token anstelle des Schlüssels als Authorization: Bearer.",
    description:
      "Mint dialog copy inside the OAuth client credentials detail: the optional client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "Fertig",
    description: "Mint dialog close button (key-shown step)",
  },
  "apiKeys.copied": {
    message: "In die Zwischenablage kopiert",
    description:
      "Toast on a successful copy (the key, the example request, the client ID or the secret)",
  },
  "apiKeys.copyError": {
    message: "Kopieren in die Zwischenablage fehlgeschlagen",
    description: "Toast on a failed copy",
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
