import type { TranslationEntry } from "@/i18n";

const deApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API-Schlüssel",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Maschinen-Zugangsdaten für Skripte und Agents. Jeder Schlüssel darf nur die für ihn gewählten Berechtigungen nutzen, nie mehr, als die aktuelle Mitgliedsrolle der Person erlaubt, die ihn erstellt hat, und nur in diesem Workspace. Du siehst und verwaltest die Schlüssel, die du erstellt hast; ein Admin kann außerdem die Schlüssel aller Mitglieder sehen und widerrufen.",
    description:
      "Settings API Keys section card description: what a key is, that it uses only its chosen permissions within its creator's current role in this workspace (w9/m496), and who sees which keys: every member their own, as on Render's Account Settings, and an Admin every member's too (w9/m539)",
  },
  "apiKeys.cliGuide": {
    message: "Die CLI einrichten.",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.scopeLabel": {
    message: "Welche API-Schlüssel angezeigt werden",
    description:
      "Accessible name of an Admin's switch between their own API keys and every member's (w9/m539)",
  },
  "apiKeys.scopeOwn": {
    message: "Deine Schlüssel",
    description: "API Keys switch option: the keys the caller created",
  },
  "apiKeys.scopeAll": {
    message: "Schlüssel aller Mitglieder",
    description:
      "API Keys switch option, an Admin's only: every member's keys in the workspace",
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
      "Sende ihn bei jeder Anfrage als Authorization: Bearer. Der Schlüssel darf nur die Berechtigungen nutzen, die du für ihn gewählt hast, nie mehr, als deine aktuelle Mitgliedsrolle in diesem Workspace erlaubt, behandle ihn also wie ein Passwort.",
    description:
      "Mint dialog copy under the key: how to send it, and that it uses only the chosen permissions, capped by the creator's current member role in this workspace (w9/m496)",
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
  "apiKeys.revokeConfirmBodyOther": {
    message:
      "{creator} hat diesen Schlüssel erstellt. Alles, was sich mit ihm authentifiziert, funktioniert sofort nicht mehr. Das lässt sich nicht rückgängig machen.",
    description:
      "Revoke-confirmation dialog body for another member's key in an Admin's every-member view; {creator} is who created it",
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
  "apiKeys.colPermissions": {
    message: "Berechtigungen",
    description: "API Keys table column header: what each key may do (w9/m496)",
  },
  "apiKeys.permSummaryAll": {
    message: "Alle",
    description:
      "Key row permissions summary: the key has every permission its creator's member role allows (Render's All)",
  },
  "apiKeys.permSummaryReadOnly": {
    message: "Nur lesen",
    description:
      "Key row permissions summary: every permission the key has is at Read, so it can change nothing",
  },
  "apiKeys.permSummaryNone": {
    message: "Keine Berechtigungen",
    description:
      "Key row permissions summary: the key has no permissions (Clear all)",
  },
  "apiKeys.permSummaryCustom": {
    message: "Individuell",
    description:
      "Key row permissions summary: the key has a chosen set of permissions; followed by their count",
  },
  "apiKeys.editPermissionsNamed": {
    message: "Berechtigungen von {name} ändern",
    description:
      "Gear button aria-label on a key row; opens the permissions editor",
  },
  "apiKeys.editPermissionsTitle": {
    message: "Berechtigungen bearbeiten",
    description: "Permissions editor dialog title",
  },
  "apiKeys.editPermissionsDescription": {
    message:
      "Ändere, was {name} nutzen darf, und klicke dann auf Änderungen speichern. Die neuen Berechtigungen gelten sofort.",
    description:
      "Permissions editor dialog description; {name} is the key's name",
  },
  "apiKeys.editPermissionsCancel": {
    message: "Abbrechen",
    description: "Permissions editor dialog cancel button",
  },
  "apiKeys.editPermissionsSave": {
    message: "Änderungen speichern",
    description: "Permissions editor dialog save button (Render's words)",
  },
  "apiKeys.permissionsSaved": {
    message: "Berechtigungen von {name} aktualisiert",
    description: "Toast after a key's permissions are saved",
  },
  "apiKeys.permissionsSaveError": {
    message: "Berechtigungen von {name} konnten nicht aktualisiert werden",
    description:
      "Toast when saving a key's permissions fails (the server's reason is shown instead when it gives one)",
  },
  "apiKeys.permIntro": {
    message: "Wähle die Berechtigungen, die dieser API-Schlüssel nutzen darf.",
    description:
      "Permission picker heading in the create dialog and the permissions editor (Render's words)",
  },
  "apiKeys.permPresetAll": {
    message: "Alle",
    description:
      "Picker preset button: grant every permission the member role allows (Render's All)",
  },
  "apiKeys.permPresetReadOnly": {
    message: "Nur lesen",
    description:
      "Picker preset button: grant Read on every permission that offers it (Render's Read only)",
  },
  "apiKeys.permPresetClear": {
    message: "Alle entfernen",
    description:
      "Picker preset button: remove every permission from the key (Render's Clear all)",
  },
  "apiKeys.permAllHelp": {
    message:
      "Alle gewährt jede Berechtigung, die die Mitgliedsrolle der erstellenden Person erlaubt, für jede Ressource, und folgt dieser Rolle, wenn sie sich ändert.",
    description: "Picker note while the All preset is selected: what All means",
  },
  "apiKeys.permNoneHelp": {
    message:
      "Keine Berechtigungen ausgewählt. Der Schlüssel kann sich nur selbst ausweisen.",
    description: "Picker note when every permission is None (Clear all)",
  },
  "apiKeys.permAreaToggle": {
    message: "Berechtigungen für {area} gewähren",
    description:
      "Switch aria-label on one area of the picker; {area} is Render's area name, e.g. Services (kept in English)",
  },
  "apiKeys.permLevelNone": {
    message: "Keine",
    description: "Picker option on a permission: the key gets none of it",
  },
  "apiKeys.permLevelGrants": {
    message: "{level} gewährt:",
    description:
      "Picker label over the actions a permission's chosen level grants; {level} is Render's level name, e.g. Read & write (kept in English)",
  },
  "apiKeys.permRequiresRole.viewer": {
    message: "{level} erfordert die Rolle Betrachter.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Viewer",
  },
  "apiKeys.permRequiresRole.contributor": {
    message: "{level} erfordert die Rolle Contributor.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Contributor",
  },
  "apiKeys.permRequiresRole.developer": {
    message: "{level} erfordert die Rolle Entwickler.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Developer",
  },
  "apiKeys.permRequiresRole.admin": {
    message: "{level} erfordert die Rolle Admin.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Admin",
  },
  "apiKeys.permRequiresHigherRole": {
    message: "{level} erfordert eine höhere Mitgliedsrolle.",
    description:
      "Reason beside a level the viewer's role cannot grant, when bex-api names a role the dashboard does not know",
  },
  "apiKeys.permCatalogLoading": {
    message: "Berechtigungen werden geladen…",
    description:
      "Accessible name of the picker while its permission list loads",
  },
  "apiKeys.permCatalogErrorCreate": {
    message:
      "Die Liste der Berechtigungen konnte nicht geladen werden. Ein Schlüssel, den du jetzt erstellst, erhält alle Berechtigungen.",
    description:
      "Create dialog note when the permission list fails to load: creating still works and grants All",
  },
  "apiKeys.permCatalogErrorEdit": {
    message: "Die Liste der Berechtigungen konnte nicht geladen werden.",
    description:
      "Permissions editor note when the permission list fails to load, beside a Try again button",
  },
  "apiKeys.permSummaryCount_one": {
    message: "{count} Berechtigung",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
  "apiKeys.permSummaryCount_other": {
    message: "{count} Berechtigungen",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
};

export default deApiKeys;
