import type { TranslationEntry } from "@/i18n";

const enRegistryCredentials: Record<string, TranslationEntry> = {
  "registryCredentials.title": {
    message: "Registry Credentials",
    description:
      "Settings Integrations → Registry Credentials section card title",
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
    description:
      "Registry Credentials state when the caller lacks permission (403)",
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
    message: "Add Credential",
    description: "Add Credential dialog title (Render's)",
  },
  "registryCredentials.createDescription": {
    message: "Learn more about storing external {link}.",
    description:
      "Add Credential dialog description; {link} is createDescriptionLink, linking bex's private-registry docs",
  },
  "registryCredentials.fieldHost": {
    message: "Registry host",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldUsername": {
    message: "Username",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "Personal Access Token",
    description: "Credential secret field label",
  },
  "registryCredentials.hostInvalid": {
    message:
      "Enter a registry host such as ghcr.io or registry.example.com:5000, without a path.",
    description:
      "Inline error under the Registry host when it is not a bare host[:port] (w21/053)",
  },
  "registryCredentials.hostTooLong": {
    message: "At most 253 bytes",
    description: "Inline error under an over-long Registry host (w21/053)",
  },
  "registryCredentials.usernameControl": {
    message: "Must not contain control characters",
    description:
      "Inline error under a registry username with a control character (w21/053)",
  },
  "registryCredentials.usernameTooLong": {
    message: "At most 253 bytes",
    description: "Inline error under an over-long registry username (w21/053)",
  },
  "registryCredentials.authTokenTooLong": {
    message: "At most 64 KiB",
    description:
      "Inline error under a registry password or token over the 64 KiB limit (w21/053)",
  },
  "registryCredentials.fieldName": {
    message: "Name",
    description: "Credential name field label (required)",
  },
  "registryCredentials.createCancel": {
    message: "Cancel",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "Add Credential",
    description: "Add Credential dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "Added a credential for {host}",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "Couldn't add the registry credential",
    description: "Toast on a failed create",
  },
  "registryCredentials.delete": {
    message: "Delete",
    description: "Row action / confirmation button to delete a credential",
  },
  "registryCredentials.deleteNamed": {
    message: "Delete registry credential {name}",
    description:
      "Row delete button aria-label identifying the registry credential",
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
      "Rename the credential, change its username, or rotate the token. The registry can't be changed.",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "Save changes",
    description: "Edit-credential dialog submit button",
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
      "Only a credential no service uses can be deleted — remove it from every service first, or the delete is refused. This can't be undone.",
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
    description:
      "Expiry-status badge for a past-expiry credential (w2/m14/t007)",
  },
  "registryCredentials.expiringSoon": {
    message: "Expiring soon",
    description: "Expiry-status badge for a credential nearing expiry",
  },
  "registryCredentials.expiresOn": {
    message: "Expires {date}",
    description:
      "Expiry-status text for an active credential with a future expiry",
  },
  "registryCredentials.neverExpires": {
    message: "Never expires",
    description: "Expiry-status text for a credential with no expiry set",
  },
  "registryCredentials.createDescriptionLink": {
    message: "registry credentials",
    description: "Link text inside createDescription",
  },
  "registryCredentials.fieldNamePlaceholder": {
    message: "My credential",
    description: "Credential name field placeholder",
  },
  "registryCredentials.fieldRegistry": {
    message: "Registry",
    description: "Add Credential dialog Registry select label",
  },
  "registryCredentials.fieldRegistryPlaceholder": {
    message: "Select a registry",
    description: "Registry select placeholder before a registry is chosen",
  },
  "registryCredentials.fieldHostHint": {
    message: "The host at the start of your image URL, without a path.",
    description:
      "Hint under the Registry host field (Google Artifact and AWS ECR only)",
  },
  "registryCredentials.hostRegistryMismatch": {
    message: "Enter a {registry} host, such as {example}.",
    description:
      "Inline error under a Registry host that belongs to another registry; {registry} is the chosen registry's name, {example} an example host",
  },
  "registryCredentials.fieldAuthTokenOrPassword": {
    message: "Personal Access Token or Password",
    description:
      "Credential secret field label for Docker, which also takes a password",
  },
  "registryCredentials.helpGithub": {
    message:
      "You’ll need to create a personal access token with the required {permission} permission in {link} {path}.",
    description:
      "Help under the Registry select for GitHub; {permission} is read:packages, {link} links GitHub's new-token page, {path} is GitHub's menu path",
  },
  "registryCredentials.helpGitlab": {
    message:
      "You’ll need to create a personal access token with the required {permission} permission in {link} by navigating to {path}.",
    description:
      "Help under the Registry select for GitLab; {permission} is read registry, {link} links GitLab's token page, {path} is GitLab's menu path",
  },
  "registryCredentials.helpDocker": {
    message:
      "Enter your Docker Hub username and password, or create an access token with {permission} access permission to use in place of your password by visiting {link} {path}.",
    description:
      "Help under the Registry select for Docker; {permission} is read, {link} links Docker Hub's security settings, {path} is Docker Hub's menu path",
  },
  "registryCredentials.helpGoogle": {
    message:
      "You’ll need to create a service account with the required {permission} permission in {link}.",
    description:
      "Help under the Registry select for Google Artifact; {permission} is the reader role, {link} links Google Cloud's service account page",
  },
  "registryCredentials.helpEcr": {
    message:
      "The Username is the AWS Account ID that owns the ECR repository. The Personal Access Token will come from running the {command} command. Refer to {link} for more information.",
    description:
      "Help under the Registry select for AWS ECR; {command} is the aws CLI command, {link} is helpEcrLink",
  },
  "registryCredentials.helpEcrLink": {
    message: "the AWS documentation",
    description: "Link text to the AWS ECR authentication docs inside helpEcr",
  },
  "registryCredentials.fieldRegistryImmutable": {
    message: "The registry can't be changed after creation.",
    description: "Hint under the read-only Registry in the edit dialog",
  },
};

export default enRegistryCredentials;
