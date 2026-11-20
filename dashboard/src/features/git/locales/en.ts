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
      "Connect the GitHub accounts and organizations this workspace deploys from. You'll be redirected to GitHub.",
    description:
      "Body text in the disconnected state: what the single connect action does (ADR078 §3b)",
  },
  "git.connectButton": {
    message: "Connect GitHub account",
    description:
      "The single connect action (ADR078 §3b, w1/m526) when the workspace has no GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.connectAnotherButton": {
    message: "Connect another account",
    description:
      "The single connect action (ADR078 §3b) once the workspace holds a GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.installAnotherButton": {
    message: "Install on another GitHub account",
    description:
      "Account chooser action that installs the bex GitHub App on an account that does not have it yet (connectGit's stateful install URL, ADR078 §3b)",
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
  "git.callbackErrorTitle": {
    message: "GitHub connection wasn't completed",
    description: "Title shown after the GitHub install callback fails",
  },
  "git.callbackErrorExpired": {
    message:
      "This connection request expired. Select Connect GitHub account to try again.",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message:
      "This connection request couldn't be verified. Select Connect GitHub account to try again.",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "This GitHub installation isn't connected to a workspace yet. GitHub can't finish the install flow for an account that already has the app. Select Connect GitHub account and choose that account.",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install): GitHub drops the state for an account that already has the App, so the recovery is the single connect action, whose account chooser lists it (ADR078 §3a/§3b)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "No GitHub account you administer was found. Check that you authorized the right GitHub user, or install the bex GitHub App on the account first.",
    description:
      "Claim-callback failure: zero candidates. Rewritten in w2/m162 — under ADR078 N:N an account connected to another workspace is claimable, so this no longer covers that case and must not imply it",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "Several GitHub accounts you administer were found and the choice couldn't be recorded. Select Connect GitHub account to start again.",
    description:
      "Claim-callback failure: ambiguity the chooser could not be offered for (the selection failed to persist). The ordinary case renders the account chooser instead",
  },
  "git.claimSelectionTitle": {
    message: "Choose a GitHub account to connect",
    description:
      "Heading of the account chooser every connect lands on (ADR078 §3b)",
  },
  "git.claimSelectionBody": {
    message:
      "These GitHub accounts have the bex GitHub App, and you administer them. Choose one to connect to this workspace, or install the App on another account.",
    description:
      "Body of the account chooser when it lists at least one installed account",
  },
  "git.claimSelectionEmpty": {
    message:
      "None of the GitHub accounts you administer has the bex GitHub App yet. Install it on one to connect it.",
    description:
      "Body of the account chooser when no account the user administers has the App yet; only the install action follows",
  },
  "git.claimSelectionGoneTitle": {
    message: "This choice has expired",
    description: "Claim selection unknown, expired, or already used",
  },
  "git.claimSelectionGoneBody": {
    message: "Select Connect GitHub account to start again.",
    description:
      "Recovery for an expired or spent account choice; the alert carries the Connect GitHub account button",
  },
  "git.selectClaimError": {
    message: "Could not connect that GitHub account.",
    description: "Toast when completing a claim selection fails",
  },
  "git.claimConnectedToast": {
    message: "{account} is connected to this workspace.",
    description:
      "Toast after a choice in the account chooser binds; {account} is the GitHub account or org login",
  },
  "git.callbackErrorGeneric": {
    message:
      "GitHub couldn't complete the connection. Select Connect GitHub account to try again.",
    description: "Generic GitHub callback failure message",
  },
  "git.callbackErrorInstallRequested": {
    message:
      "Your request to install the bex GitHub App was sent to the organization's owners. Once one of them approves it, connect again.",
    description:
      "Callback outcome for an org member's install request (setup_action=request): the org's owners must approve it first (w1/m526)",
  },
  "git.disconnectSuccess": {
    message: "GitHub disconnected.",
    description: "Toast after a successful disconnect",
  },
  "git.disconnectError": {
    message: "Couldn't disconnect GitHub.",
    description: "Toast when disconnect fails",
  },
  "git.credentialsTrigger": {
    message: "Credentials ({count})",
    description:
      "In-place credentials menu trigger on the source picker (w8/m31); {count} is the number of connected GitHub accounts",
  },
  "git.credentialsMenuLabel": {
    message: "Credentials",
    description:
      "Accessible name of the credentials menu panel on the source picker; the trigger's label without its count",
  },
  "git.credentialsAccountsHeading": {
    message: "Accounts & orgs",
    description:
      "Heading above the connected-account list in the credentials menu",
  },
  "git.repoCount_one": {
    message: "{count} repo",
    description:
      "Repo count shown next to a connected GitHub account (singular)",
  },
  "git.repoCount_other": {
    message: "{count} repos",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.openInGitHub": {
    message: "Open in GitHub",
    description: "Link/title opening a connected account's GitHub page",
  },
  "git.configureInGitHub": {
    message: "Configure in GitHub",
    description:
      "Link to GitHub's install-settings page to change repo grants (Render parity label)",
  },
  "git.disconnectAccount": {
    message: "Disconnect {account}",
    description:
      "Accessible label for a specific account's disconnect button in the credentials menu",
  },
};

export default enGit;
