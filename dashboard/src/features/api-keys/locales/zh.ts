import type { TranslationEntry } from "@/i18n";

const zhApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API 密钥",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "供脚本和智能体使用的机器凭证。每个密钥以其创建者当前的成员角色行事，且仅在此工作区内有效。整个工作区共享——任何有密钥管理权限的人都能看到这里的所有密钥，而不仅仅是自己创建的。",
    description:
      "Settings API Keys section card description: what a key is, that it acts with its creator's current role in this workspace, and that the list is shared",
  },
  "apiKeys.cliGuide": {
    message: "设置 CLI。",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.colName": {
    message: "名称",
    description: "API Keys table column header",
  },
  "apiKeys.colClientId": {
    message: "客户端 ID",
    description:
      "API Keys table column header: the key's id, which is also its OAuth2 client_id",
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
    description: "API Keys table column header: when the key was last used",
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
    message: "你没有权限管理此工作区的 API 密钥。",
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
    description: "Mint dialog title (key-shown step)",
  },
  "apiKeys.createdWarning": {
    message: "请立即复制此密钥，之后将无法再次查看。",
    description:
      "Mint dialog warning (key-shown step): the key is shown only once",
  },
  "apiKeys.fieldKey": {
    message: "API 密钥",
    description:
      "Mint dialog label for the key itself, the one string sent as Authorization: Bearer",
  },
  "apiKeys.copyKey": {
    message: "复制 API 密钥",
    description: "Copy button aria-label for the key",
  },
  "apiKeys.keyHelp": {
    message:
      "在每个请求中将它作为 Authorization: Bearer 发送。此密钥以你在该工作区当前的成员角色行事，并会随你的角色变化而变化，请像密码一样保管。",
    description:
      "Mint dialog copy under the key: how to send it, and that it acts with the creator's current member role in this workspace and follows it",
  },
  "apiKeys.curlTitle": {
    message: "示例请求",
    description:
      "Mint dialog label for the curl example that lists services with the key",
  },
  "apiKeys.copyCurl": {
    message: "复制示例请求",
    description: "Copy button aria-label for the curl example",
  },
  "apiKeys.oauthDetails": {
    message: "OAuth 客户端凭证",
    description:
      "Mint dialog title of the collapsed detail holding the client ID, the client secret and the optional token exchange",
  },
  "apiKeys.fieldClientId": {
    message: "客户端 ID",
    description:
      "Mint dialog label for the OAuth2 client_id, inside the OAuth client credentials detail",
  },
  "apiKeys.fieldSecret": {
    message: "客户端密钥",
    description:
      "Mint dialog label for the one-time client_secret, inside the OAuth client credentials detail",
  },
  "apiKeys.copyClientId": {
    message: "复制客户端 ID",
    description: "Copy button aria-label for the client_id",
  },
  "apiKeys.copyClientIdNamed": {
    message: "复制 {name} 的客户端 ID",
    description: "Copy button aria-label for one API client row's client_id",
  },
  "apiKeys.copySecret": {
    message: "复制客户端密钥",
    description: "Copy button aria-label for the client_secret",
  },
  "apiKeys.exchangeHelp": {
    message:
      "此密钥也可以作为 OAuth2 客户端使用。如果你更想使用短期令牌，可以在 {tokenEndpoint} 用客户端 ID 和客户端密钥换取访问令牌：\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\n请勿传 scope。用返回的 access_token 代替此密钥，作为 Authorization: Bearer 发送。",
    description:
      "Mint dialog copy inside the OAuth client credentials detail: the optional client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "完成",
    description: "Mint dialog close button (key-shown step)",
  },
  "apiKeys.copied": {
    message: "已复制到剪贴板",
    description:
      "Toast on a successful copy (the key, the example request, the client ID or the secret)",
  },
  "apiKeys.copyError": {
    message: "复制到剪贴板失败",
    description: "Toast on a failed copy",
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
  "apiKeys.revokeNamed": {
    message: "撤销 API 密钥 {name}",
    description: "Row revoke button aria-label identifying the API key",
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
