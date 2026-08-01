import type { TranslationEntry } from "@/i18n";

const zhApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API 密钥",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "供脚本和智能体使用的机器凭证。整个工作区共享——任何有密钥管理权限的人都能看到这里的所有密钥，而不仅仅是自己创建的。",
    description: "Settings API Keys section card description",
  },
  "apiKeys.cliGuide": {
    message: "设置 CLI。",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.colName": {
    message: "名称",
    description: "API Keys table column header",
  },
  "apiKeys.colCreated": {
    message: "创建时间",
    description: "API Keys table column header",
  },
  "apiKeys.colCreatedBy": {
    message: "创建者",
    description: "API Keys table column header — who minted the key",
  },
  "apiKeys.colLastUsed": {
    message: "最近使用",
    description:
      "API Keys table column header — when a token for the key was last used",
  },
  "apiKeys.neverUsed": {
    message: "从未使用",
    description: "API Keys last-used cell when the key has never been used",
  },
  "apiKeys.emptyTitle": {
    message: "暂无 API 密钥",
    description: "API Keys empty-state title",
  },
  "apiKeys.emptyBody": {
    message: "创建一个密钥以便脚本或智能体进行身份验证。",
    description: "API Keys empty-state body",
  },
  "apiKeys.forbiddenTitle": {
    message: "无权访问",
    description: "API Keys state when the caller lacks permission (403)",
  },
  "apiKeys.forbiddenBody": {
    message: "您没有权限管理此工作区的 API 密钥。",
    description: "API Keys forbidden-state body",
  },
  "apiKeys.errorTitle": {
    message: "无法加载 API 密钥",
    description: "API Keys generic error title",
  },
  "apiKeys.errorBody": {
    message: "出了点问题，请重试。",
    description: "API Keys generic error body",
  },
  "apiKeys.create": {
    message: "创建 API 密钥",
    description: "Button that opens the mint dialog",
  },
  "apiKeys.createTitle": {
    message: "创建 API 密钥",
    description: "Mint dialog title (name step)",
  },
  "apiKeys.createDescription": {
    message: "为这个密钥命名，方便以后识别。",
    description: "Mint dialog description (name step)",
  },
  "apiKeys.fieldName": {
    message: "名称",
    description: "Mint dialog name field label",
  },
  "apiKeys.fieldNamePlaceholder": {
    message: "例如 deploy-agent",
    description: "Mint dialog name field placeholder",
  },
  "apiKeys.createCancel": {
    message: "取消",
    description: "Mint dialog cancel button (name step)",
  },
  "apiKeys.createSubmit": {
    message: "创建",
    description: "Mint dialog submit button (name step)",
  },
  "apiKeys.createdTitle": {
    message: "API 密钥已创建",
    description: "Mint dialog title (secret-shown step)",
  },
  "apiKeys.createdWarning": {
    message: "请立即复制此密钥——之后将无法再次查看。",
    description: "Mint dialog warning (secret-shown step)",
  },
  "apiKeys.createdDone": {
    message: "完成",
    description: "Mint dialog close button (secret-shown step)",
  },
  "apiKeys.copy": {
    message: "复制",
    description: "Copy-to-clipboard icon button label",
  },
  "apiKeys.copied": {
    message: "已复制到剪贴板",
    description: "Toast on a successful secret copy",
  },
  "apiKeys.copyError": {
    message: "复制到剪贴板失败",
    description: "Toast on a failed secret copy",
  },
  "apiKeys.createSuccess": {
    message: "已创建 {name}",
    description: "Toast on a successful mint",
  },
  "apiKeys.createError": {
    message: "无法创建 {name}",
    description: "Toast on a failed mint",
  },
  "apiKeys.revoke": {
    message: "撤销",
    description: "Row action / confirmation button to revoke a key",
  },
  "apiKeys.revokeConfirmTitle": {
    message: "撤销 {name}？",
    description: "Revoke-confirmation dialog title",
  },
  "apiKeys.revokeConfirmBody": {
    message: "任何使用此密钥进行身份验证的操作都会立即失效，且无法撤回此操作。",
    description: "Revoke-confirmation dialog body",
  },
  "apiKeys.revokeCancel": {
    message: "取消",
    description: "Revoke-confirmation dialog cancel button",
  },
  "apiKeys.revokeSuccess": {
    message: "已撤销 {name}",
    description: "Toast on a successful revoke",
  },
  "apiKeys.revokeError": {
    message: "无法撤销 {name}",
    description: "Toast on a failed revoke",
  },
};

export default zhApiKeys;
