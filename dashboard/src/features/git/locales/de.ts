import type { TranslationEntry } from "@/i18n";

const deGit: Record<string, TranslationEntry> = {
  "git.title": {
    message: "GitHub verbinden",
    description: "Settings Connect GitHub card title",
  },
  "git.description": {
    message:
      "Verbinde einen GitHub-Account, um private Repositories zu deployen und Push-to-Deploy ohne Konfiguration für jedes installierte Repository zu erhalten.",
    description: "Settings Connect GitHub card description",
  },
  "git.connectedBadge": {
    message: "Verbunden",
    description: "Badge shown when GitHub is connected",
  },
  "git.disconnectedBody": {
    message:
      "Verbinde die GitHub-Accounts und -Organisationen, aus denen dieser Workspace deployt. Du wirst zu GitHub weitergeleitet.",
    description:
      "Body text in the disconnected state: what the single connect action does (ADR078 §3b)",
  },
  "git.connectButton": {
    message: "GitHub-Account verbinden",
    description:
      "The single connect action (ADR078 §3b, w1/m526) when the workspace has no GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.connectAnotherButton": {
    message: "Weiteren Account verbinden",
    description:
      "The single connect action (ADR078 §3b) once the workspace holds a GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.installAnotherButton": {
    message: "In einem anderen GitHub-Account installieren",
    description:
      "Account chooser action that installs the bex GitHub App on an account that does not have it yet (connectGit's stateful install URL, ADR078 §3b)",
  },
  "git.connectedAs": {
    message: "Verbunden als",
    description: "Label preceding the connected GitHub account login",
  },
  "git.manageAccess": {
    message: "Repository-Zugriff auf GitHub verwalten",
    description: "Link to GitHub's install-settings page to change repo grants",
  },
  "git.disconnectButton": {
    message: "Trennen",
    description: "Button that removes the GitHub connection",
  },
  "git.disconnectConfirmTitle": {
    message: "GitHub trennen?",
    description: "Confirm-dialog title for disconnecting GitHub",
  },
  "git.disconnectConfirmBody": {
    message:
      "Deployments aus privaten Repositories und automatisches Push-to-Deploy stoppen, bis du die Verbindung wiederherstellst. Die App-Installation bleibt auf GitHub bestehen, bis du sie dort entfernst.",
    description: "Confirm-dialog body for disconnecting GitHub",
  },
  "git.cancel": {
    message: "Abbrechen",
    description: "Cancel button in the disconnect confirm dialog",
  },
  "git.unavailableTitle": {
    message: "GitHub-Integration nicht konfiguriert",
    description: "State when the backend has no GitHub App configured (503)",
  },
  "git.unavailableBody": {
    message:
      "In diesem bex-Deployment ist keine GitHub App eingerichtet. Bitte den Plattformbetreiber, BEX_GITHUB_APP_ID, BEX_GITHUB_APP_PRIVATE_KEY und BEX_GITHUB_APP_SLUG zu konfigurieren.",
    description: "Body for the unavailable state",
  },
  "git.errorTitle": {
    message: "Die GitHub-Verbindung konnte nicht geladen werden",
    description: "Generic error state title",
  },
  "git.errorBody": {
    message: "Etwas ist schiefgelaufen. Versuche es gleich noch einmal.",
    description: "Generic error state body",
  },
  "git.connectError": {
    message: "Die GitHub-Verbindung konnte nicht gestartet werden.",
    description: "Toast when starting the connect flow fails",
  },
  "git.callbackErrorTitle": {
    message: "Die GitHub-Verbindung wurde nicht abgeschlossen",
    description: "Title shown after the GitHub install callback fails",
  },
  "git.callbackErrorExpired": {
    message:
      "Diese Verbindungsanfrage ist abgelaufen. Wähle „GitHub-Account verbinden“, um es erneut zu versuchen.",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message:
      "Diese Verbindungsanfrage konnte nicht verifiziert werden. Wähle „GitHub-Account verbinden“, um es erneut zu versuchen.",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "Diese GitHub-Installation ist noch mit keinem Workspace verbunden. GitHub kann den Installationsablauf für einen Account, der die App bereits hat, nicht abschließen. Wähle „GitHub-Account verbinden“ und dann diesen Account.",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install): GitHub drops the state for an account that already has the App, so the recovery is the single connect action, whose account chooser lists it (ADR078 §3a/§3b)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "Es wurde kein GitHub-Account gefunden, den du administrierst. Prüfe, ob du den richtigen GitHub-Benutzer autorisiert hast, oder installiere zuerst die bex GitHub App im Account.",
    description:
      "Claim-callback failure: zero candidates. Rewritten in w2/m162 — under ADR078 N:N an account connected to another workspace is claimable, so this no longer covers that case and must not imply it",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "Es wurden mehrere GitHub-Accounts gefunden, die du administrierst, und die Auswahl konnte nicht gespeichert werden. Wähle „GitHub-Account verbinden“, um von vorn zu beginnen.",
    description:
      "Claim-callback failure: ambiguity the chooser could not be offered for (the selection failed to persist). The ordinary case renders the account chooser instead",
  },
  "git.claimSelectionTitle": {
    message: "GitHub-Account zum Verbinden auswählen",
    description:
      "Heading of the account chooser every connect lands on (ADR078 §3b)",
  },
  "git.claimSelectionBody": {
    message:
      "Diese GitHub-Accounts haben die bex GitHub App, und du administrierst sie. Wähle einen aus, der mit diesem Workspace verbunden werden soll, oder installiere die App in einem anderen Account.",
    description:
      "Body of the account chooser when it lists at least one installed account",
  },
  "git.claimSelectionEmpty": {
    message:
      "Keiner der GitHub-Accounts, die du administrierst, hat die bex GitHub App bisher. Installiere sie in einem Account, um ihn zu verbinden.",
    description:
      "Body of the account chooser when no account the user administers has the App yet; only the install action follows",
  },
  "git.claimSelectionGoneTitle": {
    message: "Diese Auswahl ist abgelaufen",
    description: "Claim selection unknown, expired, or already used",
  },
  "git.claimSelectionGoneBody": {
    message: "Wähle „GitHub-Account verbinden“, um von vorn zu beginnen.",
    description:
      "Recovery for an expired or spent account choice; the alert carries the Connect GitHub account button",
  },
  "git.selectClaimError": {
    message: "Dieser GitHub-Account konnte nicht verbunden werden.",
    description: "Toast when completing a claim selection fails",
  },
  "git.claimConnectedToast": {
    message: "{account} ist mit diesem Workspace verbunden.",
    description:
      "Toast after a choice in the account chooser binds; {account} is the GitHub account or org login",
  },
  "git.callbackErrorGeneric": {
    message:
      "GitHub konnte die Verbindung nicht abschließen. Wähle „GitHub-Account verbinden“, um es erneut zu versuchen.",
    description: "Generic GitHub callback failure message",
  },
  "git.callbackErrorInstallRequested": {
    message:
      "Deine Anfrage zur Installation der bex GitHub App wurde an die Owner der Organisation gesendet. Sobald einer von ihnen sie genehmigt, verbinde erneut.",
    description:
      "Callback outcome for an org member's install request (setup_action=request): the org's owners must approve it first (w1/m526)",
  },
  "git.disconnectSuccess": {
    message: "GitHub getrennt.",
    description: "Toast after a successful disconnect",
  },
  "git.disconnectError": {
    message: "GitHub konnte nicht getrennt werden.",
    description: "Toast when disconnect fails",
  },
  "git.credentialsTrigger": {
    message: "Zugangsdaten ({count})",
    description:
      "In-place credentials menu trigger on the source picker (w8/m31); {count} is the number of connected GitHub accounts",
  },
  "git.credentialsMenuLabel": {
    message: "Zugangsdaten",
    description:
      "Accessible name of the credentials menu panel on the source picker; the trigger's label without its count",
  },
  "git.credentialsAccountsHeading": {
    message: "Accounts & Organisationen",
    description:
      "Heading above the connected-account list in the credentials menu",
  },
  "git.repoCount_one": {
    message: "{count} Repository",
    description:
      "Repo count shown next to a connected GitHub account (singular)",
  },
  "git.repoCount_other": {
    message: "{count} Repositories",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.openInGitHub": {
    message: "In GitHub öffnen",
    description: "Link/title opening a connected account's GitHub page",
  },
  "git.configureInGitHub": {
    message: "In GitHub konfigurieren",
    description:
      "Link to GitHub's install-settings page to change repo grants (Render parity label)",
  },
  "git.disconnectAccount": {
    message: "{account} trennen",
    description:
      "Accessible label for a specific account's disconnect button in the credentials menu",
  },
};

export default deGit;
