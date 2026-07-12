import type { TranslationEntry } from "@/i18n";

const enGit: Record<string, TranslationEntry> = {
  "git.title": {
    message: "Connect GitHub",
    description: "Settings Connect GitHub card title",
  },
  "git.description": {
    message:
      "Connect a GitHub account to deploy private repositories and get zero-config push-to-deploy for every installed repo.",
    description: "Settings Connect GitHub card description",
  },
  "git.connectedBadge": {
    message: "Connected",
    description: "Badge shown when GitHub is connected",
  },
  "git.disconnectedBody": {
    message:
      "Install the bex GitHub App on your account and choose which repositories to grant. You'll be redirected to GitHub.",
    description: "Body text in the disconnected state",
  },
  "git.connectButton": {
    message: "Connect GitHub",
    description: "Button that starts the GitHub install flow",
  },
  "git.connectedAs": {
    message: "Connected as",
    description: "Label preceding the connected GitHub account login",
  },
  "git.manageAccess": {
    message: "Manage repo access on GitHub",
    description: "Link to GitHub's install-settings page to change repo grants",
  },
  "git.disconnectButton": {
    message: "Disconnect",
    description: "Button that removes the GitHub connection",
  },
  "git.disconnectConfirmTitle": {
    message: "Disconnect GitHub?",
    description: "Confirm-dialog title for disconnecting GitHub",
  },
  "git.disconnectConfirmBody": {
    message:
      "Private-repo deploys and hands-free push-to-deploy will stop until you reconnect. The app install stays on GitHub until you remove it there.",
    description: "Confirm-dialog body for disconnecting GitHub",
  },
  "git.cancel": {
    message: "Cancel",
    description: "Cancel button in the disconnect confirm dialog",
  },
  "git.unavailableTitle": {
    message: "GitHub integration not configured",
    description: "State when the backend has no GitHub App configured (503)",
  },
  "git.unavailableBody": {
    message:
      "This bex deployment has no GitHub App set up. Ask your platform operator to configure BEX_GITHUB_APP_ID, BEX_GITHUB_APP_PRIVATE_KEY, and BEX_GITHUB_APP_SLUG.",
    description: "Body for the unavailable state",
  },
  "git.errorTitle": {
    message: "Couldn't load the GitHub connection",
    description: "Generic error state title",
  },
  "git.errorBody": {
    message: "Something went wrong. Try again in a moment.",
    description: "Generic error state body",
  },
  "git.connectError": {
    message: "Couldn't start the GitHub connection.",
    description: "Toast when starting the connect flow fails",
  },
  "git.disconnectSuccess": {
    message: "GitHub disconnected.",
    description: "Toast after a successful disconnect",
  },
  "git.disconnectError": {
    message: "Couldn't disconnect GitHub.",
    description: "Toast when disconnect fails",
  },
};

export default enGit;
