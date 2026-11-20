import type { TranslationEntry } from "@/i18n";

const zhGit: Record<string, TranslationEntry> = {
  "git.title": {
    message: "连接 GitHub",
    description: "Settings Connect GitHub card title",
  },
  "git.description": {
    message:
      "连接 GitHub 账户以部署私有仓库，并为每个已授权仓库启用零配置的推送即部署。",
    description: "Settings Connect GitHub card description",
  },
  "git.connectedBadge": {
    message: "已连接",
    description: "Badge shown when GitHub is connected",
  },
  "git.disconnectedBody": {
    message:
      "连接本工作区用于部署的 GitHub 账户和组织。你将被重定向到 GitHub。",
    description:
      "Body text in the disconnected state: what the single connect action does (ADR078 §3b)",
  },
  "git.connectButton": {
    message: "连接 GitHub 账户",
    description:
      "The single connect action (ADR078 §3b, w1/m526) when the workspace has no GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.connectAnotherButton": {
    message: "连接另一个账户",
    description:
      "The single connect action (ADR078 §3b) once the workspace holds a GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.installAnotherButton": {
    message: "安装到其他 GitHub 账户",
    description:
      "Account chooser action that installs the bex GitHub App on an account that does not have it yet (connectGit's stateful install URL, ADR078 §3b)",
  },
  "git.connectedAs": {
    message: "已连接账户",
    description: "Label preceding the connected GitHub account login",
  },
  "git.manageAccess": {
    message: "在 GitHub 上管理仓库访问",
    description: "Link to GitHub's install-settings page to change repo grants",
  },
  "git.disconnectButton": {
    message: "断开连接",
    description: "Button that removes the GitHub connection",
  },
  "git.disconnectConfirmTitle": {
    message: "断开 GitHub 连接？",
    description: "Confirm-dialog title for disconnecting GitHub",
  },
  "git.disconnectConfirmBody": {
    message:
      "在你重新连接之前，私有仓库部署和自动推送部署将停止。GitHub 上的应用安装仍会保留，直到你在那里移除它。",
    description: "Confirm-dialog body for disconnecting GitHub",
  },
  "git.cancel": {
    message: "取消",
    description: "Cancel button in the disconnect confirm dialog",
  },
  "git.unavailableTitle": {
    message: "GitHub 集成未配置",
    description: "State when the backend has no GitHub App configured (503)",
  },
  "git.unavailableBody": {
    message:
      "此 bex 部署未设置 GitHub App。请让平台管理员配置 BEX_GITHUB_APP_ID、BEX_GITHUB_APP_PRIVATE_KEY 和 BEX_GITHUB_APP_SLUG。",
    description: "Body for the unavailable state",
  },
  "git.errorTitle": {
    message: "无法加载 GitHub 连接",
    description: "Generic error state title",
  },
  "git.errorBody": {
    message: "出了点问题。请稍后重试。",
    description: "Generic error state body",
  },
  "git.connectError": {
    message: "无法开始 GitHub 连接。",
    description: "Toast when starting the connect flow fails",
  },
  "git.callbackErrorTitle": {
    message: "GitHub 连接未完成",
    description: "Title shown after the GitHub install callback fails",
  },
  "git.callbackErrorExpired": {
    message: "此连接请求已过期。请选择“连接 GitHub 账户”重试。",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message: "无法验证此连接请求。请选择“连接 GitHub 账户”重试。",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "此 GitHub 安装尚未连接到任何工作区。对于已安装该应用的账户，GitHub 无法完成安装流程。请选择“连接 GitHub 账户”，然后选择该账户。",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install): GitHub drops the state for an account that already has the App, so the recovery is the single connect action, whose account chooser lists it (ADR078 §3a/§3b)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "未找到你管理的 GitHub 账户。请确认授权了正确的 GitHub 用户，或先在该账户上安装 bex GitHub App。",
    description:
      "Claim-callback failure: zero candidates. Rewritten in w2/m162 — under ADR078 N:N an account connected to another workspace is claimable, so this no longer covers that case and must not imply it",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "找到了多个你管理的 GitHub 账户，但未能记录你的选择。请选择“连接 GitHub 账户”重新开始。",
    description:
      "Claim-callback failure: ambiguity the chooser could not be offered for (the selection failed to persist). The ordinary case renders the account chooser instead",
  },
  "git.claimSelectionTitle": {
    message: "选择要连接的 GitHub 账户",
    description:
      "Heading of the account chooser every connect lands on (ADR078 §3b)",
  },
  "git.claimSelectionBody": {
    message:
      "以下 GitHub 账户已安装 bex GitHub App，且由你管理。请选择一个连接到当前工作区，或在其他账户上安装该应用。",
    description:
      "Body of the account chooser when it lists at least one installed account",
  },
  "git.claimSelectionEmpty": {
    message:
      "你管理的 GitHub 账户都尚未安装 bex GitHub App。请先在某个账户上安装，再进行连接。",
    description:
      "Body of the account chooser when no account the user administers has the App yet; only the install action follows",
  },
  "git.claimSelectionGoneTitle": {
    message: "此选择已过期",
    description: "Claim selection unknown, expired, or already used",
  },
  "git.claimSelectionGoneBody": {
    message: "请选择“连接 GitHub 账户”重新开始。",
    description:
      "Recovery for an expired or spent account choice; the alert carries the Connect GitHub account button",
  },
  "git.selectClaimError": {
    message: "无法连接该 GitHub 账户。",
    description: "Toast when completing a claim selection fails",
  },
  "git.claimConnectedToast": {
    message: "{account} 已连接到当前工作区。",
    description:
      "Toast after a choice in the account chooser binds; {account} is the GitHub account or org login",
  },
  "git.callbackErrorGeneric": {
    message: "GitHub 无法完成连接。请选择“连接 GitHub 账户”重试。",
    description: "Generic GitHub callback failure message",
  },
  "git.callbackErrorInstallRequested": {
    message:
      "你安装 bex GitHub App 的请求已发送给该组织的所有者。待其中一位批准后，请重新连接。",
    description:
      "Callback outcome for an org member's install request (setup_action=request): the org's owners must approve it first (w1/m526)",
  },
  "git.disconnectSuccess": {
    message: "已断开 GitHub 连接。",
    description: "Toast after a successful disconnect",
  },
  "git.disconnectError": {
    message: "无法断开 GitHub 连接。",
    description: "Toast when disconnect fails",
  },
  "git.credentialsTrigger": {
    message: "凭据（{count}）",
    description:
      "In-place credentials menu trigger on the source picker (w8/m31); {count} is the number of connected GitHub accounts",
  },
  "git.credentialsMenuLabel": {
    message: "凭据",
    description:
      "Accessible name of the credentials menu panel on the source picker; the trigger's label without its count",
  },
  "git.credentialsAccountsHeading": {
    message: "账户与组织",
    description:
      "Heading above the connected-account list in the credentials menu",
  },
  "git.repoCount_other": {
    message: "{count} 个仓库",
    description: "Repo count shown next to a connected GitHub account",
  },
  "git.openInGitHub": {
    message: "在 GitHub 中打开",
    description: "Link/title opening a connected account's GitHub page",
  },
  "git.configureInGitHub": {
    message: "在 GitHub 中配置",
    description:
      "Link to GitHub's install-settings page to change repo grants (Render parity label)",
  },
  "git.disconnectAccount": {
    message: "断开 {account}",
    description:
      "Accessible label for a specific account's disconnect button in the credentials menu",
  },
};

export default zhGit;
