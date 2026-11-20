import type { TranslationEntry } from "@/i18n";

const ptGit: Record<string, TranslationEntry> = {
  "git.title": {
    message: "Conectar o GitHub",
    description: "Settings Connect GitHub card title",
  },
  "git.description": {
    message:
      "Conecte uma conta do GitHub para fazer deploy de repositórios privados e ter push-to-deploy sem configuração em todos os repositórios instalados.",
    description: "Settings Connect GitHub card description",
  },
  "git.connectedBadge": {
    message: "Conectado",
    description: "Badge shown when GitHub is connected",
  },
  "git.disconnectedBody": {
    message:
      "Conecte as contas e organizações do GitHub de onde este workspace faz deploy. Você será redirecionado ao GitHub.",
    description:
      "Body text in the disconnected state: what the single connect action does (ADR078 §3b)",
  },
  "git.connectButton": {
    message: "Conectar conta do GitHub",
    description:
      "The single connect action (ADR078 §3b, w1/m526) when the workspace has no GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.connectAnotherButton": {
    message: "Conectar outra conta",
    description:
      "The single connect action (ADR078 §3b) once the workspace holds a GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.installAnotherButton": {
    message: "Instalar em outra conta do GitHub",
    description:
      "Account chooser action that installs the bex GitHub App on an account that does not have it yet (connectGit's stateful install URL, ADR078 §3b)",
  },
  "git.connectedAs": {
    message: "Conectado como",
    description: "Label preceding the connected GitHub account login",
  },
  "git.manageAccess": {
    message: "Gerenciar o acesso aos repositórios no GitHub",
    description: "Link to GitHub's install-settings page to change repo grants",
  },
  "git.disconnectButton": {
    message: "Desconectar",
    description: "Button that removes the GitHub connection",
  },
  "git.disconnectConfirmTitle": {
    message: "Desconectar o GitHub?",
    description: "Confirm-dialog title for disconnecting GitHub",
  },
  "git.disconnectConfirmBody": {
    message:
      "Os deploys de repositórios privados e o push-to-deploy automático deixarão de funcionar até você reconectar. A instalação do app permanece no GitHub até você removê-la lá.",
    description: "Confirm-dialog body for disconnecting GitHub",
  },
  "git.cancel": {
    message: "Cancelar",
    description: "Cancel button in the disconnect confirm dialog",
  },
  "git.unavailableTitle": {
    message: "Integração com o GitHub não configurada",
    description: "State when the backend has no GitHub App configured (503)",
  },
  "git.unavailableBody": {
    message:
      "Esta instalação do bex não tem um GitHub App configurado. Peça ao operador da sua plataforma para configurar BEX_GITHUB_APP_ID, BEX_GITHUB_APP_PRIVATE_KEY e BEX_GITHUB_APP_SLUG.",
    description: "Body for the unavailable state",
  },
  "git.errorTitle": {
    message: "Não foi possível carregar a conexão com o GitHub",
    description: "Generic error state title",
  },
  "git.errorBody": {
    message: "Algo deu errado. Tente novamente em instantes.",
    description: "Generic error state body",
  },
  "git.connectError": {
    message: "Não foi possível iniciar a conexão com o GitHub.",
    description: "Toast when starting the connect flow fails",
  },
  "git.callbackErrorTitle": {
    message: "A conexão com o GitHub não foi concluída",
    description: "Title shown after the GitHub install callback fails",
  },
  "git.callbackErrorExpired": {
    message:
      "Esta solicitação de conexão expirou. Selecione Conectar conta do GitHub para tentar novamente.",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message:
      "Não foi possível verificar esta solicitação de conexão. Selecione Conectar conta do GitHub para tentar novamente.",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "Esta instalação do GitHub ainda não está conectada a um workspace. O GitHub não consegue concluir o fluxo de instalação para uma conta que já tem o app. Selecione Conectar conta do GitHub e escolha essa conta.",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install): GitHub drops the state for an account that already has the App, so the recovery is the single connect action, whose account chooser lists it (ADR078 §3a/§3b)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "Nenhuma conta do GitHub que você administra foi encontrada. Verifique se você autorizou o usuário certo do GitHub ou instale primeiro o GitHub App do bex na conta.",
    description:
      "Claim-callback failure: zero candidates. Rewritten in w2/m162 — under ADR078 N:N an account connected to another workspace is claimable, so this no longer covers that case and must not imply it",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "Foram encontradas várias contas do GitHub que você administra e a escolha não pôde ser registrada. Selecione Conectar conta do GitHub para começar de novo.",
    description:
      "Claim-callback failure: ambiguity the chooser could not be offered for (the selection failed to persist). The ordinary case renders the account chooser instead",
  },
  "git.claimSelectionTitle": {
    message: "Escolha uma conta do GitHub para conectar",
    description:
      "Heading of the account chooser every connect lands on (ADR078 §3b)",
  },
  "git.claimSelectionBody": {
    message:
      "Estas contas do GitHub têm o GitHub App do bex e você as administra. Escolha uma para conectar a este workspace ou instale o App em outra conta.",
    description:
      "Body of the account chooser when it lists at least one installed account",
  },
  "git.claimSelectionEmpty": {
    message:
      "Nenhuma das contas do GitHub que você administra tem o GitHub App do bex ainda. Instale-o em uma para conectá-la.",
    description:
      "Body of the account chooser when no account the user administers has the App yet; only the install action follows",
  },
  "git.claimSelectionGoneTitle": {
    message: "Esta escolha expirou",
    description: "Claim selection unknown, expired, or already used",
  },
  "git.claimSelectionGoneBody": {
    message: "Selecione Conectar conta do GitHub para começar de novo.",
    description:
      "Recovery for an expired or spent account choice; the alert carries the Connect GitHub account button",
  },
  "git.selectClaimError": {
    message: "Não foi possível conectar essa conta do GitHub.",
    description: "Toast when completing a claim selection fails",
  },
  "git.claimConnectedToast": {
    message: "{account} está conectada a este workspace.",
    description:
      "Toast after a choice in the account chooser binds; {account} is the GitHub account or org login",
  },
  "git.callbackErrorGeneric": {
    message:
      "O GitHub não conseguiu concluir a conexão. Selecione Conectar conta do GitHub para tentar novamente.",
    description: "Generic GitHub callback failure message",
  },
  "git.callbackErrorInstallRequested": {
    message:
      "Sua solicitação para instalar o GitHub App do bex foi enviada aos proprietários da organização. Quando um deles aprovar, conecte novamente.",
    description:
      "Callback outcome for an org member's install request (setup_action=request): the org's owners must approve it first (w1/m526)",
  },
  "git.disconnectSuccess": {
    message: "GitHub desconectado.",
    description: "Toast after a successful disconnect",
  },
  "git.disconnectError": {
    message: "Não foi possível desconectar o GitHub.",
    description: "Toast when disconnect fails",
  },
  "git.credentialsTrigger": {
    message: "Credenciais ({count})",
    description:
      "In-place credentials menu trigger on the source picker (w8/m31); {count} is the number of connected GitHub accounts",
  },
  "git.credentialsMenuLabel": {
    message: "Credenciais",
    description:
      "Accessible name of the credentials menu panel on the source picker; the trigger's label without its count",
  },
  "git.credentialsAccountsHeading": {
    message: "Contas e organizações",
    description:
      "Heading above the connected-account list in the credentials menu",
  },
  "git.repoCount_one": {
    message: "{count} repositório",
    description:
      "Repo count shown next to a connected GitHub account (singular)",
  },
  "git.repoCount_many": {
    message: "{count} de repositórios",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.repoCount_other": {
    message: "{count} repositórios",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.openInGitHub": {
    message: "Abrir no GitHub",
    description: "Link/title opening a connected account's GitHub page",
  },
  "git.configureInGitHub": {
    message: "Configurar no GitHub",
    description:
      "Link to GitHub's install-settings page to change repo grants (Render parity label)",
  },
  "git.disconnectAccount": {
    message: "Desconectar {account}",
    description:
      "Accessible label for a specific account's disconnect button in the credentials menu",
  },
};

export default ptGit;
