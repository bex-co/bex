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
    message: "Registry-Zugangsdaten hinzufügen",
    description: "Create dialog title",
  },
  "registryCredentials.createDescription": {
    message:
      "Sicher gespeichert. Das Passwort bzw. der Token wird danach nie wieder angezeigt.",
    description: "Create dialog description",
  },
  "registryCredentials.fieldHost": {
    message: "Registry-Host",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldHostPlaceholder": {
    message: "z. B. ghcr.io, docker.io, registry.gitlab.com",
    description: "Create dialog host field placeholder",
  },
  "registryCredentials.fieldUsername": {
    message: "Benutzername",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "Passwort oder Zugriffstoken",
    description: "Create dialog secret field label",
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
    message: "Anzeigename",
    description: "Create dialog optional display-name field label",
  },
  "registryCredentials.fieldNameOptional": {
    message: "(optional — standardmäßig der Host)",
    description: "Create dialog display-name field's optional hint",
  },
  "registryCredentials.createCancel": {
    message: "Abbrechen",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "Hinzufügen",
    description: "Create dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "Zugangsdaten für {host} hinzugefügt",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "Zugangsdaten für {host} konnten nicht hinzugefügt werden",
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
      "Benenne die Zugangsdaten um, ändere den Benutzernamen oder rotiere den Token. Der Registry-Host kann nicht geändert werden.",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "Änderungen speichern",
    description: "Edit-credential dialog submit button",
  },
  "registryCredentials.fieldHostImmutable": {
    message: "Der Registry-Host kann nach dem Erstellen nicht geändert werden.",
    description: "Hint under the read-only host field in the edit dialog",
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
};

export default deRegistryCredentials;
