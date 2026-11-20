import type { TranslationEntry } from "@/i18n";

const koGit: Record<string, TranslationEntry> = {
  "git.title": {
    message: "GitHub 연결",
    description: "Settings Connect GitHub card title",
  },
  "git.description": {
    message:
      "GitHub 계정을 연결하면 비공개 저장소를 배포하고, 설치된 모든 저장소에서 별도 설정 없이 푸시 즉시 배포할 수 있습니다.",
    description: "Settings Connect GitHub card description",
  },
  "git.connectedBadge": {
    message: "연결됨",
    description: "Badge shown when GitHub is connected",
  },
  "git.disconnectedBody": {
    message:
      "이 워크스페이스가 배포에 사용하는 GitHub 계정과 조직을 연결하세요. GitHub로 이동합니다.",
    description:
      "Body text in the disconnected state: what the single connect action does (ADR078 §3b)",
  },
  "git.connectButton": {
    message: "GitHub 계정 연결",
    description:
      "The single connect action (ADR078 §3b, w1/m526) when the workspace has no GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.connectAnotherButton": {
    message: "다른 계정 연결",
    description:
      "The single connect action (ADR078 §3b) once the workspace holds a GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.installAnotherButton": {
    message: "다른 GitHub 계정에 설치",
    description:
      "Account chooser action that installs the bex GitHub App on an account that does not have it yet (connectGit's stateful install URL, ADR078 §3b)",
  },
  "git.connectedAs": {
    message: "연결된 계정",
    description: "Label preceding the connected GitHub account login",
  },
  "git.manageAccess": {
    message: "GitHub에서 저장소 액세스 관리",
    description: "Link to GitHub's install-settings page to change repo grants",
  },
  "git.disconnectButton": {
    message: "연결 해제",
    description: "Button that removes the GitHub connection",
  },
  "git.disconnectConfirmTitle": {
    message: "GitHub 연결을 해제할까요?",
    description: "Confirm-dialog title for disconnecting GitHub",
  },
  "git.disconnectConfirmBody": {
    message:
      "다시 연결하기 전까지 비공개 저장소 배포와 자동 푸시 배포가 중단됩니다. 앱 설치는 GitHub에서 직접 제거하기 전까지 남아 있습니다.",
    description: "Confirm-dialog body for disconnecting GitHub",
  },
  "git.cancel": {
    message: "취소",
    description: "Cancel button in the disconnect confirm dialog",
  },
  "git.unavailableTitle": {
    message: "GitHub 연동이 구성되지 않았습니다",
    description: "State when the backend has no GitHub App configured (503)",
  },
  "git.unavailableBody": {
    message:
      "이 bex 배포에는 GitHub 앱이 설정되어 있지 않습니다. 플랫폼 운영자에게 BEX_GITHUB_APP_ID, BEX_GITHUB_APP_PRIVATE_KEY, BEX_GITHUB_APP_SLUG를 구성해 달라고 요청하세요.",
    description: "Body for the unavailable state",
  },
  "git.errorTitle": {
    message: "GitHub 연결을 불러오지 못했습니다",
    description: "Generic error state title",
  },
  "git.errorBody": {
    message: "문제가 발생했습니다. 잠시 후 다시 시도하세요.",
    description: "Generic error state body",
  },
  "git.connectError": {
    message: "GitHub 연결을 시작하지 못했습니다.",
    description: "Toast when starting the connect flow fails",
  },
  "git.callbackErrorTitle": {
    message: "GitHub 연결이 완료되지 않았습니다",
    description: "Title shown after the GitHub install callback fails",
  },
  "git.callbackErrorExpired": {
    message:
      "이 연결 요청이 만료되었습니다. GitHub 계정 연결을 선택하여 다시 시도하세요.",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message:
      "이 연결 요청을 확인할 수 없습니다. GitHub 계정 연결을 선택하여 다시 시도하세요.",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "이 GitHub 설치는 아직 워크스페이스에 연결되지 않았습니다. 이미 앱이 설치된 계정에서는 GitHub가 설치 과정을 완료할 수 없습니다. GitHub 계정 연결을 선택한 다음 해당 계정을 고르세요.",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install): GitHub drops the state for an account that already has the App, so the recovery is the single connect action, whose account chooser lists it (ADR078 §3a/§3b)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "관리 중인 GitHub 계정을 찾을 수 없습니다. 올바른 GitHub 사용자로 승인했는지 확인하거나, 먼저 해당 계정에 bex GitHub 앱을 설치하세요.",
    description:
      "Claim-callback failure: zero candidates. Rewritten in w2/m162 — under ADR078 N:N an account connected to another workspace is claimable, so this no longer covers that case and must not imply it",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "관리 중인 GitHub 계정이 여러 개 발견되었으며 선택 내용을 기록하지 못했습니다. GitHub 계정 연결을 선택하여 처음부터 다시 시작하세요.",
    description:
      "Claim-callback failure: ambiguity the chooser could not be offered for (the selection failed to persist). The ordinary case renders the account chooser instead",
  },
  "git.claimSelectionTitle": {
    message: "연결할 GitHub 계정 선택",
    description:
      "Heading of the account chooser every connect lands on (ADR078 §3b)",
  },
  "git.claimSelectionBody": {
    message:
      "다음 GitHub 계정에는 bex GitHub 앱이 설치되어 있으며 내가 관리하는 계정입니다. 이 워크스페이스에 연결할 계정을 선택하거나 다른 계정에 앱을 설치하세요.",
    description:
      "Body of the account chooser when it lists at least one installed account",
  },
  "git.claimSelectionEmpty": {
    message:
      "내가 관리하는 GitHub 계정 중 아직 bex GitHub 앱이 설치된 계정이 없습니다. 연결하려면 계정에 앱을 설치하세요.",
    description:
      "Body of the account chooser when no account the user administers has the App yet; only the install action follows",
  },
  "git.claimSelectionGoneTitle": {
    message: "이 선택이 만료되었습니다",
    description: "Claim selection unknown, expired, or already used",
  },
  "git.claimSelectionGoneBody": {
    message: "GitHub 계정 연결을 선택하여 처음부터 다시 시작하세요.",
    description:
      "Recovery for an expired or spent account choice; the alert carries the Connect GitHub account button",
  },
  "git.selectClaimError": {
    message: "해당 GitHub 계정을 연결하지 못했습니다.",
    description: "Toast when completing a claim selection fails",
  },
  "git.claimConnectedToast": {
    message: "{account}이(가) 이 워크스페이스에 연결되었습니다.",
    description:
      "Toast after a choice in the account chooser binds; {account} is the GitHub account or org login",
  },
  "git.callbackErrorGeneric": {
    message:
      "GitHub에서 연결을 완료하지 못했습니다. GitHub 계정 연결을 선택하여 다시 시도하세요.",
    description: "Generic GitHub callback failure message",
  },
  "git.callbackErrorInstallRequested": {
    message:
      "bex GitHub 앱 설치 요청을 조직 소유자에게 보냈습니다. 소유자 중 한 명이 승인하면 다시 연결하세요.",
    description:
      "Callback outcome for an org member's install request (setup_action=request): the org's owners must approve it first (w1/m526)",
  },
  "git.disconnectSuccess": {
    message: "GitHub 연결을 해제했습니다.",
    description: "Toast after a successful disconnect",
  },
  "git.disconnectError": {
    message: "GitHub 연결을 해제하지 못했습니다.",
    description: "Toast when disconnect fails",
  },
  "git.credentialsTrigger": {
    message: "자격 증명 ({count})",
    description:
      "In-place credentials menu trigger on the source picker (w8/m31); {count} is the number of connected GitHub accounts",
  },
  "git.credentialsMenuLabel": {
    message: "자격 증명",
    description:
      "Accessible name of the credentials menu panel on the source picker; the trigger's label without its count",
  },
  "git.credentialsAccountsHeading": {
    message: "계정 및 조직",
    description:
      "Heading above the connected-account list in the credentials menu",
  },
  "git.repoCount_other": {
    message: "저장소 {count}개",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.openInGitHub": {
    message: "GitHub에서 열기",
    description: "Link/title opening a connected account's GitHub page",
  },
  "git.configureInGitHub": {
    message: "GitHub에서 구성",
    description:
      "Link to GitHub's install-settings page to change repo grants (Render parity label)",
  },
  "git.disconnectAccount": {
    message: "{account} 연결 해제",
    description:
      "Accessible label for a specific account's disconnect button in the credentials menu",
  },
};

export default koGit;
