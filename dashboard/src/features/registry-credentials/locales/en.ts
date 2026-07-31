import type { TranslationEntry } from "@/i18n";

const enRegistryCredentials: Record<string, TranslationEntry> = {
  "registryCredentials.title": {
    message: "Registry Credentials",
    description: "Settings Integrations → Registry Credentials section card title",
  },
  "registryCredentials.description": {
    message:
      "Credentials for private external image registries (Docker Hub, GHCR, GitLab Container Registry, ECR, etc.), so an existing-image service can pull from them.",
    description: "Settings Registry Credentials section card description",
  },
  "registryCredentials.colName": {
    message: "Name",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colUsername": {
    message: "Username",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colStatus": {
    message: "Expiry",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colCreated": {
    message: "Created",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.emptyTitle": {
    message: "No registry credentials",
    description: "Registry Credentials empty-state title",
  },
  "registryCredentials.emptyBody": {
    message:
      "Add a credential so a service can pull an image from a private registry.",
    description: "Registry Credentials empty-state body",
  },
  "registryCredentials.forbiddenTitle": {
    message: "Not authorized",
    description: "Registry Credentials state when the caller lacks permission (403)",
  },
  "registryCredentials.forbiddenBody": {
    message:
      "You don't have permission to manage this workspace's registry credentials.",
    description: "Registry Credentials forbidden-state body",
  },
  "registryCredentials.errorTitle": {
    message: "Couldn't load registry credentials",
    description: "Registry Credentials generic error title",
  },
  "registryCredentials.errorBody": {
    message: "Something went wrong. Please try again.",
    description: "Registry Credentials generic error body",
  },
  "registryCredentials.create": {
    message: "Add credential",
    description: "Button that opens the create dialog",
  },
  "registryCredentials.createTitle": {
    message: "Add registry credential",
    description: "Create dialog title",
  },
  "registryCredentials.createDescription": {
    message:
      "Stored securely. The password/token is never shown again after this.",
    description: "Create dialog description",
  },
  "registryCredentials.fieldHost": {
    message: "Registry host",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldHostPlaceholder": {
    message: "e.g. ghcr.io, docker.io, registry.gitlab.com",
    description: "Create dialog host field placeholder",
  },
  "registryCredentials.fieldUsername": {
    message: "Username",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "Password or access token",
    description: "Create dialog secret field label",
  },
  "registryCredentials.fieldName": {
    message: "Display name",
    description: "Create dialog optional display-name field label",
  },
  "registryCredentials.fieldNameOptional": {
    message: "(optional — defaults to the host)",
    description: "Create dialog display-name field's optional hint",
  },
  "registryCredentials.createCancel": {
    message: "Cancel",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "Add",
    description: "Create dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "Added a credential for {host}",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "Couldn't add the credential for {host}",
    description: "Toast on a failed create",
  },
  "registryCredentials.delete": {
    message: "Delete",
    description: "Row action / confirmation button to delete a credential",
  },
  "registryCredentials.edit": {
    message: "Edit",
    description: "Row action to open the edit-credential dialog",
  },
  "registryCredentials.editTitle": {
    message: "Edit registry credential",
    description: "Edit-credential dialog title",
  },
  "registryCredentials.editDescription": {
    message:
      "Rename the credential, change its username, or rotate the token. The registry host can't be changed.",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "Save changes",
    description: "Edit-credential dialog submit button",
  },
  "registryCredentials.fieldHostImmutable": {
    message: "The registry host can't be changed after creation.",
    description: "Hint under the read-only host field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeep": {
    message: "Leave blank to keep the current token",
    description: "Placeholder for the token field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeepHint": {
    message:
      "The stored token is never shown. Enter a new value only to rotate it.",
    description: "Hint under the token field in the edit dialog",
  },
  "registryCredentials.updateSuccess": {
    message: "Registry credential updated.",
    description: "Toast after an edit succeeds",
  },
  "registryCredentials.updateError": {
    message: "Couldn't update the registry credential.",
    description: "Toast after an edit fails",
  },
  "registryCredentials.deleteConfirmTitle": {
    message: "Delete {name}?",
    description: "Delete-confirmation dialog title",
  },
  "registryCredentials.deleteConfirmBody": {
    message:
      "Services already using this credential's pull secret are unaffected until their next deploy re-resolves it. This can't be undone.",
    description: "Delete-confirmation dialog body",
  },
  "registryCredentials.deleteCancel": {
    message: "Cancel",
    description: "Delete-confirmation dialog cancel button",
  },
  "registryCredentials.deleteSuccess": {
    message: "Deleted {name}",
    description: "Toast on a successful delete",
  },
  "registryCredentials.deleteError": {
    message: "Couldn't delete {name}",
    description: "Toast on a failed delete",
  },
  "registryCredentials.expired": {
    message: "Expired",
    description: "Expiry-status badge for a past-expiry credential (w2/m14/t007)",
  },
  "registryCredentials.expiringSoon": {
    message: "Expiring soon",
    description: "Expiry-status badge for a credential nearing expiry",
  },
  "registryCredentials.expiresOn": {
    message: "Expires {date}",
    description: "Expiry-status text for an active credential with a future expiry",
  },
  "registryCredentials.neverExpires": {
    message: "Never expires",
    description: "Expiry-status text for a credential with no expiry set",
  },
};

export default enRegistryCredentials;
