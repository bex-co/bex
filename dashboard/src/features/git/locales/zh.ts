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
    description: "Callback error shown when signed state is missing or invalid",
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
};

export default zhGit;
