import type { TranslationEntry } from "@/i18n";

const zh: Record<string, TranslationEntry> = {
  "connectedAgents.title": {
    message: "已连接的代理",
    description: "Connected Agents settings card title",
  },
  "connectedAgents.description": {
    message:
      "您已授权代表您行事的 OAuth 客户端。撤销后将立即使其访问令牌失效。",
    description: "Connected Agents settings card description",
  },
  "connectedAgents.colClient": {
    message: "客户端",
    description: "Connected Agents table column: client name",
  },
  "connectedAgents.colScopes": {
    message: "权限范围",
    description: "Connected Agents table column: granted scopes",
  },
  "connectedAgents.colGranted": {
    message: "授权时间",
    description: "Connected Agents table column: grant date",
  },
  "connectedAgents.emptyTitle": {
    message: "暂无已连接的代理",
    description: "Connected Agents empty state title",
  },
  "connectedAgents.emptyBody": {
    message: "您尚未授权任何 OAuth 客户端。",
    description: "Connected Agents empty state body",
  },
  "connectedAgents.errorTitle": {
    message: "无法加载已连接的代理",
    description: "Connected Agents generic error state title",
  },
  "connectedAgents.errorBody": {
    message: "出了点问题，请稍后再试。",
    description: "Connected Agents generic error state body",
  },
  "connectedAgents.revoke": {
    message: "撤销",
    description: "Connected Agents row revoke button label",
  },
  "connectedAgents.revokeConfirmTitle": {
    message: "撤销「{name}」的访问权限？",
    description: "Connected Agents revoke confirmation dialog title",
  },
  "connectedAgents.revokeConfirmBody": {
    message:
      "此操作将立即使该客户端持有的所有访问令牌失效。它需要重新获得授权才能再次代表您行事。",
    description: "Connected Agents revoke confirmation dialog body",
  },
  "connectedAgents.revokeCancel": {
    message: "取消",
    description: "Connected Agents revoke confirmation dialog cancel button",
  },
  "connectedAgents.revokeSuccess": {
    message: "已撤销「{name}」的访问权限",
    description: "Connected Agents revoke success toast",
  },
  "connectedAgents.revokeError": {
    message: "无法撤销「{name}」的访问权限",
    description: "Connected Agents revoke failure toast",
  },
};

export default zh;
