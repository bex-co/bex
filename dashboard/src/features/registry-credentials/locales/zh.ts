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
    message: "你没有权限管理此工作区的镜像仓库凭据。",
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
    message: "添加凭据",
    description: "Add Credential dialog title (Render's)",
  },
  "registryCredentials.createDescription": {
    message: "了解如何存储外部{link}。",
    description:
      "Add Credential dialog description; {link} is createDescriptionLink, linking bex's private-registry docs",
  },
  "registryCredentials.fieldHost": {
    message: "仓库主机",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldUsername": {
    message: "用户名",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "个人访问令牌",
    description: "Credential secret field label",
  },
  "registryCredentials.hostInvalid": {
    message:
      "请输入镜像仓库主机，例如 ghcr.io 或 registry.example.com:5000，不要带路径。",
    description:
      "Inline error under the Registry host when it is not a bare host[:port] (w21/053)",
  },
  "registryCredentials.hostTooLong": {
    message: "最多 253 字节",
    description: "Inline error under an over-long Registry host (w21/053)",
  },
  "registryCredentials.usernameControl": {
    message: "不能包含控制字符",
    description:
      "Inline error under a registry username with a control character (w21/053)",
  },
  "registryCredentials.usernameTooLong": {
    message: "最多 253 字节",
    description: "Inline error under an over-long registry username (w21/053)",
  },
  "registryCredentials.authTokenTooLong": {
    message: "最多 64 KiB",
    description:
      "Inline error under a registry password or token over the 64 KiB limit (w21/053)",
  },
  "registryCredentials.fieldName": {
    message: "名称",
    description: "Credential name field label (required)",
  },
  "registryCredentials.createCancel": {
    message: "取消",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "添加凭据",
    description: "Add Credential dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "已添加 {host} 的凭据",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "无法添加镜像仓库凭据",
    description: "Toast on a failed create",
  },
  "registryCredentials.delete": {
    message: "删除",
    description: "Row action / confirmation button to delete a credential",
  },
  "registryCredentials.deleteNamed": {
    message: "删除镜像仓库凭据 {name}",
    description:
      "Row delete button aria-label identifying the registry credential",
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
    message: "重命名凭据、修改用户名或轮换令牌。镜像仓库不可更改。",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "保存更改",
    description: "Edit-credential dialog submit button",
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
      "只能删除未被任何服务使用的凭据——请先从所有服务中移除它，否则删除会被拒绝。此操作无法撤销。",
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
  "registryCredentials.createDescriptionLink": {
    message: "镜像仓库凭据",
    description: "Link text inside createDescription",
  },
  "registryCredentials.fieldNamePlaceholder": {
    message: "我的凭据",
    description: "Credential name field placeholder",
  },
  "registryCredentials.fieldRegistry": {
    message: "镜像仓库",
    description: "Add Credential dialog Registry select label",
  },
  "registryCredentials.fieldRegistryPlaceholder": {
    message: "选择镜像仓库",
    description: "Registry select placeholder before a registry is chosen",
  },
  "registryCredentials.fieldHostHint": {
    message: "镜像 URL 开头的主机，不含路径。",
    description:
      "Hint under the Registry host field (Google Artifact and AWS ECR only)",
  },
  "registryCredentials.hostRegistryMismatch": {
    message: "请输入 {registry} 的主机，例如 {example}。",
    description:
      "Inline error under a Registry host that belongs to another registry; {registry} is the chosen registry's name, {example} an example host",
  },
  "registryCredentials.fieldAuthTokenOrPassword": {
    message: "个人访问令牌或密码",
    description:
      "Credential secret field label for Docker, which also takes a password",
  },
  "registryCredentials.helpGithub": {
    message:
      "你需要在 {link} {path} 中创建一个具有 {permission} 权限的个人访问令牌。",
    description:
      "Help under the Registry select for GitHub; {permission} is read:packages, {link} links GitHub's new-token page, {path} is GitHub's menu path",
  },
  "registryCredentials.helpGitlab": {
    message:
      "你需要在 {link} 中前往 {path}，创建一个具有 {permission} 权限的个人访问令牌。",
    description:
      "Help under the Registry select for GitLab; {permission} is read registry, {link} links GitLab's token page, {path} is GitLab's menu path",
  },
  "registryCredentials.helpDocker": {
    message:
      "输入你的 Docker Hub 用户名和密码，或前往 {link} {path} 创建一个具有 {permission} 访问权限的访问令牌来代替密码。",
    description:
      "Help under the Registry select for Docker; {permission} is read, {link} links Docker Hub's security settings, {path} is Docker Hub's menu path",
  },
  "registryCredentials.helpGoogle": {
    message: "你需要在 {link} 中创建一个具有 {permission} 权限的服务账号。",
    description:
      "Help under the Registry select for Google Artifact; {permission} is the reader role, {link} links Google Cloud's service account page",
  },
  "registryCredentials.helpEcr": {
    message:
      "用户名是拥有该 ECR 仓库的 AWS 账号 ID。个人访问令牌是运行 {command} 命令的输出。详情请参阅{link}。",
    description:
      "Help under the Registry select for AWS ECR; {command} is the aws CLI command, {link} is helpEcrLink",
  },
  "registryCredentials.helpEcrLink": {
    message: "AWS 文档",
    description: "Link text to the AWS ECR authentication docs inside helpEcr",
  },
  "registryCredentials.fieldRegistryImmutable": {
    message: "镜像仓库创建后不可更改。",
    description: "Hint under the read-only Registry in the edit dialog",
  },
};

export default zhRegistryCredentials;
