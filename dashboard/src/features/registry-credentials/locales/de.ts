import type { TranslationEntry } from "@/i18n";

const deRegistryCredentials: Record<string, TranslationEntry> = {
  "registryCredentials.title": {
    message: "Registry-Zugangsdaten",
    description:
      "Settings Integrations → Registry Credentials section card title",
  },
  "registryCredentials.description": {
    message:
      "Zugangsdaten für private externe Image-Registries (Docker Hub, GHCR, GitLab Container Registry, ECR usw.), damit ein Service mit vorhandenem Image von dort pullen kann.",
    description: "Settings Registry Credentials section card description",
  },
  "registryCredentials.colName": {
    message: "Name",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colUsername": {
    message: "Benutzername",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colStatus": {
    message: "Ablauf",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colCreated": {
    message: "Erstellt",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.emptyTitle": {
    message: "Keine Registry-Zugangsdaten",
    description: "Registry Credentials empty-state title",
  },
  "registryCredentials.emptyBody": {
    message:
      "Füge Zugangsdaten hinzu, damit ein Service ein Image aus einer privaten Registry pullen kann.",
    description: "Registry Credentials empty-state body",
  },
  "registryCredentials.forbiddenTitle": {
    message: "Nicht berechtigt",
    description:
      "Registry Credentials state when the caller lacks permission (403)",
  },
  "registryCredentials.forbiddenBody": {
    message:
      "Du hast keine Berechtigung, die Registry-Zugangsdaten dieses Workspace zu verwalten.",
    description: "Registry Credentials forbidden-state body",
  },
  "registryCredentials.errorTitle": {
    message: "Registry-Zugangsdaten konnten nicht geladen werden",
    description: "Registry Credentials generic error title",
  },
  "registryCredentials.errorBody": {
    message: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
    description: "Registry Credentials generic error body",
  },
  "registryCredentials.create": {
    message: "Zugangsdaten hinzufügen",
    description: "Button that opens the create dialog",
  },
  "registryCredentials.createTitle": {
    message: "Zugangsdaten hinzufügen",
    description: "Add Credential dialog title (Render's)",
  },
  "registryCredentials.createDescription": {
    message: "Mehr über das Speichern externer {link}.",
    description:
      "Add Credential dialog description; {link} is createDescriptionLink, linking bex's private-registry docs",
  },
  "registryCredentials.fieldHost": {
    message: "Registry-Host",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldUsername": {
    message: "Benutzername",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "Persönliches Zugriffstoken",
    description: "Credential secret field label",
  },
  "registryCredentials.hostInvalid": {
    message:
      "Gib einen Registry-Host wie ghcr.io oder registry.example.com:5000 ein, ohne Pfad.",
    description:
      "Inline error under the Registry host when it is not a bare host[:port] (w21/053)",
  },
  "registryCredentials.hostTooLong": {
    message: "Höchstens 253 Bytes",
    description: "Inline error under an over-long Registry host (w21/053)",
  },
  "registryCredentials.usernameControl": {
    message: "Darf keine Steuerzeichen enthalten",
    description:
      "Inline error under a registry username with a control character (w21/053)",
  },
  "registryCredentials.usernameTooLong": {
    message: "Höchstens 253 Bytes",
    description: "Inline error under an over-long registry username (w21/053)",
  },
  "registryCredentials.authTokenTooLong": {
    message: "Höchstens 64 KiB",
    description:
      "Inline error under a registry password or token over the 64 KiB limit (w21/053)",
  },
  "registryCredentials.fieldName": {
    message: "Name",
    description: "Credential name field label (required)",
  },
  "registryCredentials.createCancel": {
    message: "Abbrechen",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "Zugangsdaten hinzufügen",
    description: "Add Credential dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "Zugangsdaten für {host} hinzugefügt",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "Die Registry-Zugangsdaten konnten nicht hinzugefügt werden",
    description: "Toast on a failed create",
  },
  "registryCredentials.delete": {
    message: "Löschen",
    description: "Row action / confirmation button to delete a credential",
  },
  "registryCredentials.deleteNamed": {
    message: "Registry-Zugangsdaten {name} löschen",
    description:
      "Row delete button aria-label identifying the registry credential",
  },
  "registryCredentials.edit": {
    message: "Bearbeiten",
    description: "Row action to open the edit-credential dialog",
  },
  "registryCredentials.editTitle": {
    message: "Registry-Zugangsdaten bearbeiten",
    description: "Edit-credential dialog title",
  },
  "registryCredentials.editDescription": {
    message:
      "Benenne die Zugangsdaten um, ändere den Benutzernamen oder rotiere den Token. Die Registry kann nicht geändert werden.",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "Änderungen speichern",
    description: "Edit-credential dialog submit button",
  },
  "registryCredentials.fieldAuthTokenKeep": {
    message: "Leer lassen, um den aktuellen Token zu behalten",
    description: "Placeholder for the token field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeepHint": {
    message:
      "Der gespeicherte Token wird nie angezeigt. Gib nur dann einen neuen Wert ein, wenn du ihn rotieren möchtest.",
    description: "Hint under the token field in the edit dialog",
  },
  "registryCredentials.updateSuccess": {
    message: "Registry-Zugangsdaten aktualisiert.",
    description: "Toast after an edit succeeds",
  },
  "registryCredentials.updateError": {
    message: "Registry-Zugangsdaten konnten nicht aktualisiert werden.",
    description: "Toast after an edit fails",
  },
  "registryCredentials.deleteConfirmTitle": {
    message: "{name} löschen?",
    description: "Delete-confirmation dialog title",
  },
  "registryCredentials.deleteConfirmBody": {
    message:
      "Nur Zugangsdaten, die kein Service verwendet, können gelöscht werden — entferne sie zuerst aus jedem Service, sonst wird das Löschen abgelehnt. Das lässt sich nicht rückgängig machen.",
    description: "Delete-confirmation dialog body",
  },
  "registryCredentials.deleteCancel": {
    message: "Abbrechen",
    description: "Delete-confirmation dialog cancel button",
  },
  "registryCredentials.deleteSuccess": {
    message: "{name} gelöscht",
    description: "Toast on a successful delete",
  },
  "registryCredentials.deleteError": {
    message: "{name} konnte nicht gelöscht werden",
    description: "Toast on a failed delete",
  },
  "registryCredentials.expired": {
    message: "Abgelaufen",
    description:
      "Expiry-status badge for a past-expiry credential (w2/m14/t007)",
  },
  "registryCredentials.expiringSoon": {
    message: "Läuft bald ab",
    description: "Expiry-status badge for a credential nearing expiry",
  },
  "registryCredentials.expiresOn": {
    message: "Läuft ab am {date}",
    description:
      "Expiry-status text for an active credential with a future expiry",
  },
  "registryCredentials.neverExpires": {
    message: "Läuft nie ab",
    description: "Expiry-status text for a credential with no expiry set",
  },
  "registryCredentials.createDescriptionLink": {
    message: "Registry-Zugangsdaten",
    description: "Link text inside createDescription",
  },
  "registryCredentials.fieldNamePlaceholder": {
    message: "Meine Zugangsdaten",
    description: "Credential name field placeholder",
  },
  "registryCredentials.fieldRegistry": {
    message: "Registry",
    description: "Add Credential dialog Registry select label",
  },
  "registryCredentials.fieldRegistryPlaceholder": {
    message: "Registry auswählen",
    description: "Registry select placeholder before a registry is chosen",
  },
  "registryCredentials.fieldHostHint": {
    message: "Der Host am Anfang deiner Image-URL, ohne Pfad.",
    description:
      "Hint under the Registry host field (Google Artifact and AWS ECR only)",
  },
  "registryCredentials.hostRegistryMismatch": {
    message: "Gib einen {registry}-Host ein, zum Beispiel {example}.",
    description:
      "Inline error under a Registry host that belongs to another registry; {registry} is the chosen registry's name, {example} an example host",
  },
  "registryCredentials.fieldAuthTokenOrPassword": {
    message: "Persönliches Zugriffstoken oder Passwort",
    description:
      "Credential secret field label for Docker, which also takes a password",
  },
  "registryCredentials.helpGithub": {
    message:
      "Erstelle unter {link} {path} ein persönliches Zugriffstoken mit der Berechtigung {permission}.",
    description:
      "Help under the Registry select for GitHub; {permission} is read:packages, {link} links GitHub's new-token page, {path} is GitHub's menu path",
  },
  "registryCredentials.helpGitlab": {
    message:
      "Erstelle in {link} unter {path} ein persönliches Zugriffstoken mit der Berechtigung {permission}.",
    description:
      "Help under the Registry select for GitLab; {permission} is read registry, {link} links GitLab's token page, {path} is GitLab's menu path",
  },
  "registryCredentials.helpDocker": {
    message:
      "Gib deinen Docker-Hub-Benutzernamen und dein Passwort ein, oder erstelle unter {link} {path} ein Zugriffstoken mit der Berechtigung {permission}, das du statt des Passworts verwendest.",
    description:
      "Help under the Registry select for Docker; {permission} is read, {link} links Docker Hub's security settings, {path} is Docker Hub's menu path",
  },
  "registryCredentials.helpGoogle": {
    message:
      "Erstelle in {link} ein Dienstkonto mit der Berechtigung {permission}.",
    description:
      "Help under the Registry select for Google Artifact; {permission} is the reader role, {link} links Google Cloud's service account page",
  },
  "registryCredentials.helpEcr": {
    message:
      "Der Benutzername ist die ID des AWS-Kontos, dem das ECR-Repository gehört. Das persönliche Zugriffstoken ist die Ausgabe des Befehls {command}. Mehr dazu in {link}.",
    description:
      "Help under the Registry select for AWS ECR; {command} is the aws CLI command, {link} is helpEcrLink",
  },
  "registryCredentials.helpEcrLink": {
    message: "der AWS-Dokumentation",
    description: "Link text to the AWS ECR authentication docs inside helpEcr",
  },
  "registryCredentials.fieldRegistryImmutable": {
    message: "Die Registry kann nach dem Erstellen nicht geändert werden.",
    description: "Hint under the read-only Registry in the edit dialog",
  },
};

export default deRegistryCredentials;
