import type { TranslationEntry } from "@/i18n";

const zhRegistryCredentials: Record<string, TranslationEntry> = {
  "registryCredentials.title": {
    message: "镜像仓库凭据",
    description:
      "Settings Integrations → Registry Credentials section card title",
  },
  "registryCredentials.description": {
    message:
      "私有外部镜像仓库（Docker Hub、GHCR、GitLab Container Registry、ECR 等）的凭据，使已有镜像的服务可以从中拉取镜像。",
    description: "Settings Registry Credentials section card description",
  },
  "registryCredentials.colName": {
    message: "名称",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colUsername": {
    message: "用户名",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colStatus": {
    message: "到期状态",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colCreated": {
    message: "创建时间",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.emptyTitle": {
    message: "暂无镜像仓库凭据",
    description: "Registry Credentials empty-state title",
  },
  "registryCredentials.emptyBody": {
    message: "添加凭据后，服务即可从私有镜像仓库拉取镜像。",
    description: "Registry Credentials empty-state body",
  },
  "registryCredentials.forbiddenTitle": {
    message: "无权限",
    description:
      "Registry Credentials state when the caller lacks permission (403)",
  },
  "registryCredentials.forbiddenBody": {
    message: "您没有权限管理此工作区的镜像仓库凭据。",
    description: "Registry Credentials forbidden-state body",
  },
  "registryCredentials.errorTitle": {
    message: "无法加载镜像仓库凭据",
    description: "Registry Credentials generic error title",
  },
  "registryCredentials.errorBody": {
    message: "出了点问题，请重试。",
    description: "Registry Credentials generic error body",
  },
  "registryCredentials.create": {
    message: "添加凭据",
    description: "Button that opens the create dialog",
  },
  "registryCredentials.createTitle": {
    message: "添加镜像仓库凭据",
    description: "Create dialog title",
  },
  "registryCredentials.createDescription": {
    message: "将被安全存储。密码或令牌在此之后将不再显示。",
    description: "Create dialog description",
  },
  "registryCredentials.fieldHost": {
    message: "仓库主机",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldHostPlaceholder": {
    message: "例如 ghcr.io、docker.io、registry.gitlab.com",
    description: "Create dialog host field placeholder",
  },
  "registryCredentials.fieldUsername": {
    message: "用户名",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "密码或访问令牌",
    description: "Create dialog secret field label",
  },
  "registryCredentials.fieldName": {
    message: "显示名称",
    description: "Create dialog optional display-name field label",
  },
  "registryCredentials.fieldNameOptional": {
    message: "（可选——默认为仓库主机）",
    description: "Create dialog display-name field's optional hint",
  },
  "registryCredentials.createCancel": {
    message: "取消",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "添加",
    description: "Create dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "已添加 {host} 的凭据",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "无法添加 {host} 的凭据",
    description: "Toast on a failed create",
  },
  "registryCredentials.delete": {
    message: "删除",
    description: "Row action / confirmation button to delete a credential",
  },
  "registryCredentials.edit": {
    message: "编辑",
    description: "Row action to open the edit-credential dialog",
  },
  "registryCredentials.editTitle": {
    message: "编辑镜像仓库凭据",
    description: "Edit-credential dialog title",
  },
  "registryCredentials.editDescription": {
    message: "重命名凭据、修改用户名或轮换令牌。仓库主机创建后不可更改。",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "保存更改",
    description: "Edit-credential dialog submit button",
  },
  "registryCredentials.fieldHostImmutable": {
    message: "仓库主机创建后不可更改。",
    description: "Hint under the read-only host field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeep": {
    message: "留空以保留当前令牌",
    description: "Placeholder for the token field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeepHint": {
    message: "存储的令牌不会显示。仅在需要轮换时输入新值。",
    description: "Hint under the token field in the edit dialog",
  },
  "registryCredentials.updateSuccess": {
    message: "镜像仓库凭据已更新。",
    description: "Toast after an edit succeeds",
  },
  "registryCredentials.updateError": {
    message: "无法更新镜像仓库凭据。",
    description: "Toast after an edit fails",
  },
  "registryCredentials.deleteConfirmTitle": {
    message: "删除 {name}？",
    description: "Delete-confirmation dialog title",
  },
  "registryCredentials.deleteConfirmBody": {
    message:
      "已在使用此凭据拉取密钥的服务不会立即受影响，直到其下次部署重新解析凭据。此操作无法撤销。",
    description: "Delete-confirmation dialog body",
  },
  "registryCredentials.deleteCancel": {
    message: "取消",
    description: "Delete-confirmation dialog cancel button",
  },
  "registryCredentials.deleteSuccess": {
    message: "已删除 {name}",
    description: "Toast on a successful delete",
  },
  "registryCredentials.deleteError": {
    message: "无法删除 {name}",
    description: "Toast on a failed delete",
  },
  "registryCredentials.expired": {
    message: "已过期",
    description:
      "Expiry-status badge for a past-expiry credential (w2/m14/t007)",
  },
  "registryCredentials.expiringSoon": {
    message: "即将过期",
    description: "Expiry-status badge for a credential nearing expiry",
  },
  "registryCredentials.expiresOn": {
    message: "{date} 到期",
    description:
      "Expiry-status text for an active credential with a future expiry",
  },
  "registryCredentials.neverExpires": {
    message: "永不过期",
    description: "Expiry-status text for a credential with no expiry set",
  },
};

export default zhRegistryCredentials;
