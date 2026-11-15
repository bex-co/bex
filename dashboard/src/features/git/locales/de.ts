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
      "Installiere die bex GitHub App in deinem Account und wähle aus, welchen Repositories du Zugriff gewährst. Du wirst zu GitHub weitergeleitet.",
    description: "Body text in the disconnected state",
  },
  "git.connectButton": {
    message: "GitHub verbinden",
    description: "Button that starts the GitHub install flow",
  },
  "git.connectAnotherButton": {
    message: "Weiteren Account verbinden",
    description:
      "Button that connects an additional GitHub account/org to the workspace",
  },
  "git.claimButton": {
    message: "Installierten Account übernehmen",
    description:
      "Button starting the ADR075 §3a claim flow: bind a GitHub account where the app is ALREADY installed (GitHub strips the install URL's state for those)",
  },
  "git.claimHint": {
    message:
      "Die bex GitHub App ist in diesem Account schon installiert? Nutze stattdessen „Übernehmen“. GitHub zeigt für einen Account, der sie hat, „Configure“ statt „Install“ an, und dieser Weg führt nie zurück zu bex.",
    description:
      "Hint under the connect/claim buttons explaining when to use claim",
  },
  "git.claimError": {
    message: "Die Übernahme des GitHub-Accounts konnte nicht gestartet werden.",
    description: "Toast when starting the claim flow fails",
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
      "Diese Verbindungsanfrage ist abgelaufen. Wähle „GitHub verbinden“, um es erneut zu versuchen.",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message:
      "Diese Verbindungsanfrage konnte nicht verifiziert werden. Wähle „GitHub verbinden“, um es erneut zu versuchen.",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "Diese GitHub-Installation ist noch mit keinem Workspace verbunden. GitHub kann den Installationsablauf für einen Account, der die App bereits hat, nicht abschließen – nutze unten „Installierten Account übernehmen“.",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install) — points at the claim flow, which is the path that works for already-installed accounts (ADR075 §3a)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "Es wurde kein GitHub-Account gefunden, den du administrierst. Prüfe, ob du den richtigen GitHub-Benutzer autorisiert hast, oder installiere zuerst die bex GitHub App im Account.",
    description:
      "Claim-callback failure: zero candidates. Rewritten in w2/m162 — under ADR078 N:N an account connected to another workspace is claimable, so this no longer covers that case and must not imply it",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "Es wurden mehrere GitHub-Accounts gefunden, die du administrierst, und die Auswahl konnte nicht gespeichert werden. Wähle „Installierten Account übernehmen“, um von vorn zu beginnen.",
    description:
      "Claim-callback failure: ambiguity the picker could not be offered for (the selection failed to persist). The ordinary ambiguous case now renders the account picker instead",
  },
  "git.claimSelectionTitle": {
    message: "GitHub-Account zum Verbinden auswählen",
    description: "Heading of the deferred claim account picker (ADR078 §3a)",
  },
  "git.claimSelectionBody": {
    message:
      "Du administrierst mehr als einen GitHub-Account. Wähle den aus, der mit diesem Workspace verbunden werden soll.",
    description: "Body of the deferred claim account picker",
  },
  "git.claimSelectionGoneTitle": {
    message: "Diese Auswahl ist abgelaufen",
    description: "Claim selection unknown, expired, or already used",
  },
  "git.claimSelectionGoneBody": {
    message:
      "Wähle unten „Installierten Account übernehmen“, um von vorn zu beginnen.",
    description: "Recovery for an expired or spent claim selection",
  },
  "git.selectClaimError": {
    message: "Dieser GitHub-Account konnte nicht verbunden werden.",
    description: "Toast when completing a claim selection fails",
  },
  "git.callbackErrorGeneric": {
    message:
      "GitHub konnte die Verbindung nicht abschließen. Wähle „GitHub verbinden“, um es erneut zu versuchen.",
    description: "Generic GitHub callback failure message",
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
