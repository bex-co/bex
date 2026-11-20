import type { TranslationEntry } from "@/i18n";

const esGit: Record<string, TranslationEntry> = {
  "git.title": {
    message: "Conectar GitHub",
    description: "Settings Connect GitHub card title",
  },
  "git.description": {
    message:
      "Conecta una cuenta de GitHub para desplegar repositorios privados y obtener despliegue automático con cada push, sin configuración, en todos los repositorios instalados.",
    description: "Settings Connect GitHub card description",
  },
  "git.connectedBadge": {
    message: "Conectado",
    description: "Badge shown when GitHub is connected",
  },
  "git.disconnectedBody": {
    message:
      "Conecta las cuentas y organizaciones de GitHub desde las que despliega este espacio de trabajo. Se te redirigirá a GitHub.",
    description:
      "Body text in the disconnected state: what the single connect action does (ADR078 §3b)",
  },
  "git.connectButton": {
    message: "Conectar cuenta de GitHub",
    description:
      "The single connect action (ADR078 §3b, w1/m526) when the workspace has no GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.connectAnotherButton": {
    message: "Conectar otra cuenta",
    description:
      "The single connect action (ADR078 §3b) once the workspace holds a GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.installAnotherButton": {
    message: "Instalar en otra cuenta de GitHub",
    description:
      "Account chooser action that installs the bex GitHub App on an account that does not have it yet (connectGit's stateful install URL, ADR078 §3b)",
  },
  "git.connectedAs": {
    message: "Conectado como",
    description: "Label preceding the connected GitHub account login",
  },
  "git.manageAccess": {
    message: "Administrar el acceso a repositorios en GitHub",
    description: "Link to GitHub's install-settings page to change repo grants",
  },
  "git.disconnectButton": {
    message: "Desconectar",
    description: "Button that removes the GitHub connection",
  },
  "git.disconnectConfirmTitle": {
    message: "¿Desconectar GitHub?",
    description: "Confirm-dialog title for disconnecting GitHub",
  },
  "git.disconnectConfirmBody": {
    message:
      "Los despliegues de repositorios privados y el despliegue automático con cada push se detendrán hasta que vuelvas a conectar. La instalación de la app permanece en GitHub hasta que la quites allí.",
    description: "Confirm-dialog body for disconnecting GitHub",
  },
  "git.cancel": {
    message: "Cancelar",
    description: "Cancel button in the disconnect confirm dialog",
  },
  "git.unavailableTitle": {
    message: "La integración con GitHub no está configurada",
    description: "State when the backend has no GitHub App configured (503)",
  },
  "git.unavailableBody": {
    message:
      "Este despliegue de bex no tiene una GitHub App configurada. Pide al operador de tu plataforma que configure BEX_GITHUB_APP_ID, BEX_GITHUB_APP_PRIVATE_KEY y BEX_GITHUB_APP_SLUG.",
    description: "Body for the unavailable state",
  },
  "git.errorTitle": {
    message: "No se pudo cargar la conexión con GitHub",
    description: "Generic error state title",
  },
  "git.errorBody": {
    message: "Algo salió mal. Inténtalo de nuevo en un momento.",
    description: "Generic error state body",
  },
  "git.connectError": {
    message: "No se pudo iniciar la conexión con GitHub.",
    description: "Toast when starting the connect flow fails",
  },
  "git.callbackErrorTitle": {
    message: "La conexión con GitHub no se completó",
    description: "Title shown after the GitHub install callback fails",
  },
  "git.callbackErrorExpired": {
    message:
      "Esta solicitud de conexión caducó. Selecciona Conectar cuenta de GitHub para intentarlo de nuevo.",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message:
      "No se pudo verificar esta solicitud de conexión. Selecciona Conectar cuenta de GitHub para intentarlo de nuevo.",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "Esta instalación de GitHub aún no está conectada a un espacio de trabajo. GitHub no puede completar el flujo de instalación para una cuenta que ya tiene la app. Selecciona Conectar cuenta de GitHub y elige esa cuenta.",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install): GitHub drops the state for an account that already has the App, so the recovery is the single connect action, whose account chooser lists it (ADR078 §3a/§3b)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "No se encontró ninguna cuenta de GitHub que administres. Comprueba que autorizaste al usuario de GitHub correcto o instala primero la GitHub App de bex en la cuenta.",
    description:
      "Claim-callback failure: zero candidates. Rewritten in w2/m162 — under ADR078 N:N an account connected to another workspace is claimable, so this no longer covers that case and must not imply it",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "Se encontraron varias cuentas de GitHub que administras y no se pudo registrar la elección. Selecciona Conectar cuenta de GitHub para empezar de nuevo.",
    description:
      "Claim-callback failure: ambiguity the chooser could not be offered for (the selection failed to persist). The ordinary case renders the account chooser instead",
  },
  "git.claimSelectionTitle": {
    message: "Elige una cuenta de GitHub para conectar",
    description:
      "Heading of the account chooser every connect lands on (ADR078 §3b)",
  },
  "git.claimSelectionBody": {
    message:
      "Estas cuentas de GitHub tienen la GitHub App de bex y tú las administras. Elige una para conectarla a este espacio de trabajo o instala la App en otra cuenta.",
    description:
      "Body of the account chooser when it lists at least one installed account",
  },
  "git.claimSelectionEmpty": {
    message:
      "Ninguna de las cuentas de GitHub que administras tiene todavía la GitHub App de bex. Instálala en una para conectarla.",
    description:
      "Body of the account chooser when no account the user administers has the App yet; only the install action follows",
  },
  "git.claimSelectionGoneTitle": {
    message: "Esta elección ha caducado",
    description: "Claim selection unknown, expired, or already used",
  },
  "git.claimSelectionGoneBody": {
    message: "Selecciona Conectar cuenta de GitHub para empezar de nuevo.",
    description:
      "Recovery for an expired or spent account choice; the alert carries the Connect GitHub account button",
  },
  "git.selectClaimError": {
    message: "No se pudo conectar esa cuenta de GitHub.",
    description: "Toast when completing a claim selection fails",
  },
  "git.claimConnectedToast": {
    message: "{account} está conectada a este espacio de trabajo.",
    description:
      "Toast after a choice in the account chooser binds; {account} is the GitHub account or org login",
  },
  "git.callbackErrorGeneric": {
    message:
      "GitHub no pudo completar la conexión. Selecciona Conectar cuenta de GitHub para intentarlo de nuevo.",
    description: "Generic GitHub callback failure message",
  },
  "git.callbackErrorInstallRequested": {
    message:
      "Tu solicitud para instalar la GitHub App de bex se envió a los propietarios de la organización. Cuando uno de ellos la apruebe, vuelve a conectar.",
    description:
      "Callback outcome for an org member's install request (setup_action=request): the org's owners must approve it first (w1/m526)",
  },
  "git.disconnectSuccess": {
    message: "GitHub desconectado.",
    description: "Toast after a successful disconnect",
  },
  "git.disconnectError": {
    message: "No se pudo desconectar GitHub.",
    description: "Toast when disconnect fails",
  },
  "git.credentialsTrigger": {
    message: "Credenciales ({count})",
    description:
      "In-place credentials menu trigger on the source picker (w8/m31); {count} is the number of connected GitHub accounts",
  },
  "git.credentialsMenuLabel": {
    message: "Credenciales",
    description:
      "Accessible name of the credentials menu panel on the source picker; the trigger's label without its count",
  },
  "git.credentialsAccountsHeading": {
    message: "Cuentas y organizaciones",
    description:
      "Heading above the connected-account list in the credentials menu",
  },
  "git.repoCount_one": {
    message: "{count} repositorio",
    description:
      "Repo count shown next to a connected GitHub account (singular)",
  },
  "git.repoCount_many": {
    message: "{count} de repositorios",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.repoCount_other": {
    message: "{count} repositorios",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.openInGitHub": {
    message: "Abrir en GitHub",
    description: "Link/title opening a connected account's GitHub page",
  },
  "git.configureInGitHub": {
    message: "Configurar en GitHub",
    description:
      "Link to GitHub's install-settings page to change repo grants (Render parity label)",
  },
  "git.disconnectAccount": {
    message: "Desconectar {account}",
    description:
      "Accessible label for a specific account's disconnect button in the credentials menu",
  },
};

export default esGit;
