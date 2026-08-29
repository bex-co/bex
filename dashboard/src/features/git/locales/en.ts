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
  "git.connectAnotherButton": {
    message: "Connect another account",
    description:
      "Button that connects an additional GitHub account/org to the workspace",
  },
  "git.claimButton": {
    message: "Claim installed account",
    description:
      "Button starting the ADR075 §3a claim flow: bind a GitHub account where the app is ALREADY installed (GitHub strips the install URL's state for those)",
  },
  "git.claimHint": {
    message:
      "Already installed the bex GitHub App directly on GitHub? Claim it instead — the install flow only works for accounts without the app.",
    description:
      "Hint under the connect/claim buttons explaining when to use claim",
  },
  "git.claimError": {
    message: "Couldn't start the GitHub claim.",
    description: "Toast when starting the claim flow fails",
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
      "This connection request expired. Select Connect GitHub to try again.",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message:
      "This connection request couldn't be verified. Select Connect GitHub to try again.",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "This GitHub installation isn't connected to a workspace yet. GitHub can't complete the install flow for an account that already has the app — use Claim installed account below.",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install) — points at the claim flow, which is the path that works for already-installed accounts (ADR075 §3a)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "No unconnected GitHub account you administer was found. Install the bex GitHub App on the account first, or check that you authorized the right GitHub user.",
    description: "Claim-callback failure: zero claimable installations",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "Several unconnected GitHub accounts you administer were found. Claiming binds exactly one — connect the others from their own workspaces first, then claim again.",
    description: "Claim-callback failure: more than one claimable installation",
  },
  "git.callbackErrorGeneric": {
    message:
      "GitHub couldn't complete the connection. Select Connect GitHub to try again.",
    description: "Generic GitHub callback failure message",
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
  "git.credentialsAccountsHeading": {
    message: "Accounts & orgs",
    description: "Heading above the connected-account list in the credentials menu",
  },
  "git.repoCount": {
    message: "{count} repos",
    description: "Repo count shown next to a connected GitHub account",
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
