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
      "在你的账户上安装 bex GitHub App 并选择要授权的仓库。你将被重定向到 GitHub。",
    description: "Body text in the disconnected state",
  },
  "git.connectButton": {
    message: "连接 GitHub",
    description: "Button that starts the GitHub install flow",
  },
  "git.connectAnotherButton": {
    message: "连接另一个账户",
    description:
      "Button that connects an additional GitHub account/org to the workspace",
  },
  "git.claimButton": {
    message: "认领已安装账户",
    description:
      "Button starting the ADR075 §3a claim flow: bind a GitHub account where the app is ALREADY installed (GitHub strips the install URL's state for those)",
  },
  "git.claimHint": {
    message:
      "已经直接在 GitHub 上安装了 bex GitHub App？请使用认领——安装流程只适用于尚未安装该应用的账户。",
    description:
      "Hint under the connect/claim buttons explaining when to use claim",
  },
  "git.claimError": {
    message: "无法开始 GitHub 认领流程。",
    description: "Toast when starting the claim flow fails",
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
    message: "此连接请求已过期。请选择“连接 GitHub”重试。",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message: "无法验证此连接请求。请选择“连接 GitHub”重试。",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "此 GitHub 安装尚未连接到任何工作区。对已安装应用的账户，GitHub 无法完成安装流程——请使用下方的“认领已安装账户”。",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install) — points at the claim flow, which is the path that works for already-installed accounts (ADR075 §3a)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "未找到你管理的、尚未连接的 GitHub 账户。请先在该账户上安装 bex GitHub App，或确认授权了正确的 GitHub 用户。",
    description: "Claim-callback failure: zero claimable installations",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "找到了多个你管理的、尚未连接的 GitHub 账户。认领一次只绑定一个——请先在各自的工作区连接其他账户，然后再认领。",
    description: "Claim-callback failure: more than one claimable installation",
  },
  "git.callbackErrorGeneric": {
    message: "GitHub 无法完成连接。请选择“连接 GitHub”重试。",
    description: "Generic GitHub callback failure message",
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
  "git.credentialsAccountsHeading": {
    message: "账户与组织",
    description:
      "Heading above the connected-account list in the credentials menu",
  },
  "git.repoCount": {
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
