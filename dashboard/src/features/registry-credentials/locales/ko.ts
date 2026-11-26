import type { TranslationEntry } from "@/i18n";

const koRegistryCredentials: Record<string, TranslationEntry> = {
  "registryCredentials.title": {
    message: "레지스트리 자격 증명",
    description:
      "Settings Integrations → Registry Credentials section card title",
  },
  "registryCredentials.description": {
    message:
      "비공개 외부 이미지 레지스트리(Docker Hub, GHCR, GitLab Container Registry, ECR 등)의 자격 증명입니다. 기존 이미지 서비스가 해당 레지스트리에서 이미지를 가져올 수 있게 합니다.",
    description: "Settings Registry Credentials section card description",
  },
  "registryCredentials.colName": {
    message: "이름",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colUsername": {
    message: "사용자 이름",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colStatus": {
    message: "만료",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colCreated": {
    message: "생성일",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.emptyTitle": {
    message: "레지스트리 자격 증명 없음",
    description: "Registry Credentials empty-state title",
  },
  "registryCredentials.emptyBody": {
    message:
      "서비스가 비공개 레지스트리에서 이미지를 가져올 수 있도록 자격 증명을 추가하세요.",
    description: "Registry Credentials empty-state body",
  },
  "registryCredentials.forbiddenTitle": {
    message: "권한 없음",
    description:
      "Registry Credentials state when the caller lacks permission (403)",
  },
  "registryCredentials.forbiddenBody": {
    message: "이 워크스페이스의 레지스트리 자격 증명을 관리할 권한이 없습니다.",
    description: "Registry Credentials forbidden-state body",
  },
  "registryCredentials.errorTitle": {
    message: "레지스트리 자격 증명을 불러오지 못했습니다",
    description: "Registry Credentials generic error title",
  },
  "registryCredentials.errorBody": {
    message: "문제가 발생했습니다. 다시 시도하세요.",
    description: "Registry Credentials generic error body",
  },
  "registryCredentials.create": {
    message: "자격 증명 추가",
    description: "Button that opens the create dialog",
  },
  "registryCredentials.createTitle": {
    message: "자격 증명 추가",
    description: "Add Credential dialog title (Render's)",
  },
  "registryCredentials.createDescription": {
    message: "외부 {link} 저장에 대해 자세히 알아보세요.",
    description:
      "Add Credential dialog description; {link} is createDescriptionLink, linking bex's private-registry docs",
  },
  "registryCredentials.fieldHost": {
    message: "레지스트리 호스트",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldUsername": {
    message: "사용자 이름",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "개인 액세스 토큰",
    description: "Credential secret field label",
  },
  "registryCredentials.hostInvalid": {
    message:
      "경로 없이 ghcr.io 또는 registry.example.com:5000과 같은 레지스트리 호스트를 입력하세요.",
    description:
      "Inline error under the Registry host when it is not a bare host[:port] (w21/053)",
  },
  "registryCredentials.hostTooLong": {
    message: "최대 253바이트",
    description: "Inline error under an over-long Registry host (w21/053)",
  },
  "registryCredentials.usernameControl": {
    message: "제어 문자를 포함할 수 없습니다",
    description:
      "Inline error under a registry username with a control character (w21/053)",
  },
  "registryCredentials.usernameTooLong": {
    message: "최대 253바이트",
    description: "Inline error under an over-long registry username (w21/053)",
  },
  "registryCredentials.authTokenTooLong": {
    message: "최대 64 KiB",
    description:
      "Inline error under a registry password or token over the 64 KiB limit (w21/053)",
  },
  "registryCredentials.fieldName": {
    message: "이름",
    description: "Credential name field label (required)",
  },
  "registryCredentials.createCancel": {
    message: "취소",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "자격 증명 추가",
    description: "Add Credential dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "{host}용 자격 증명을 추가했습니다",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "레지스트리 자격 증명을 추가하지 못했습니다",
    description: "Toast on a failed create",
  },
  "registryCredentials.delete": {
    message: "삭제",
    description: "Row action / confirmation button to delete a credential",
  },
  "registryCredentials.deleteNamed": {
    message: "레지스트리 자격 증명 {name} 삭제",
    description:
      "Row delete button aria-label identifying the registry credential",
  },
  "registryCredentials.edit": {
    message: "편집",
    description: "Row action to open the edit-credential dialog",
  },
  "registryCredentials.editTitle": {
    message: "레지스트리 자격 증명 편집",
    description: "Edit-credential dialog title",
  },
  "registryCredentials.editDescription": {
    message:
      "자격 증명의 이름과 사용자 이름을 변경하거나 토큰을 교체할 수 있습니다. 레지스트리는 변경할 수 없습니다.",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "변경 사항 저장",
    description: "Edit-credential dialog submit button",
  },
  "registryCredentials.fieldAuthTokenKeep": {
    message: "현재 토큰을 유지하려면 비워 두세요",
    description: "Placeholder for the token field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeepHint": {
    message:
      "저장된 토큰은 표시되지 않습니다. 토큰을 교체할 때만 새 값을 입력하세요.",
    description: "Hint under the token field in the edit dialog",
  },
  "registryCredentials.updateSuccess": {
    message: "레지스트리 자격 증명을 업데이트했습니다.",
    description: "Toast after an edit succeeds",
  },
  "registryCredentials.updateError": {
    message: "레지스트리 자격 증명을 업데이트하지 못했습니다.",
    description: "Toast after an edit fails",
  },
  "registryCredentials.deleteConfirmTitle": {
    message: "{name} 삭제",
    description: "Delete-confirmation dialog title",
  },
  "registryCredentials.deleteConfirmBody": {
    message:
      "어떤 서비스도 사용하지 않는 자격 증명만 삭제할 수 있습니다. 먼저 모든 서비스에서 제거하세요. 그렇지 않으면 삭제가 거부됩니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Delete-confirmation dialog body",
  },
  "registryCredentials.deleteCancel": {
    message: "취소",
    description: "Delete-confirmation dialog cancel button",
  },
  "registryCredentials.deleteSuccess": {
    message: "{name} 삭제 완료",
    description: "Toast on a successful delete",
  },
  "registryCredentials.deleteError": {
    message: "{name} 삭제 실패",
    description: "Toast on a failed delete",
  },
  "registryCredentials.expired": {
    message: "만료됨",
    description:
      "Expiry-status badge for a past-expiry credential (w2/m14/t007)",
  },
  "registryCredentials.expiringSoon": {
    message: "곧 만료",
    description: "Expiry-status badge for a credential nearing expiry",
  },
  "registryCredentials.expiresOn": {
    message: "만료일 {date}",
    description:
      "Expiry-status text for an active credential with a future expiry",
  },
  "registryCredentials.neverExpires": {
    message: "만료되지 않음",
    description: "Expiry-status text for a credential with no expiry set",
  },
  "registryCredentials.createDescriptionLink": {
    message: "레지스트리 자격 증명",
    description: "Link text inside createDescription",
  },
  "registryCredentials.fieldNamePlaceholder": {
    message: "내 자격 증명",
    description: "Credential name field placeholder",
  },
  "registryCredentials.fieldRegistry": {
    message: "레지스트리",
    description: "Add Credential dialog Registry select label",
  },
  "registryCredentials.fieldRegistryPlaceholder": {
    message: "레지스트리 선택",
    description: "Registry select placeholder before a registry is chosen",
  },
  "registryCredentials.fieldHostHint": {
    message: "이미지 URL 맨 앞의 호스트이며, 경로는 포함하지 않습니다.",
    description:
      "Hint under the Registry host field (Google Artifact and AWS ECR only)",
  },
  "registryCredentials.hostRegistryMismatch": {
    message: "{example}처럼 {registry} 호스트를 입력하세요.",
    description:
      "Inline error under a Registry host that belongs to another registry; {registry} is the chosen registry's name, {example} an example host",
  },
  "registryCredentials.fieldAuthTokenOrPassword": {
    message: "개인 액세스 토큰 또는 비밀번호",
    description:
      "Credential secret field label for Docker, which also takes a password",
  },
  "registryCredentials.helpGithub": {
    message:
      "{link} {path}에서 {permission} 권한이 있는 개인 액세스 토큰을 만들어야 합니다.",
    description:
      "Help under the Registry select for GitHub; {permission} is read:packages, {link} links GitHub's new-token page, {path} is GitHub's menu path",
  },
  "registryCredentials.helpGitlab": {
    message:
      "{link}에서 {path}(으)로 이동해 {permission} 권한이 있는 개인 액세스 토큰을 만들어야 합니다.",
    description:
      "Help under the Registry select for GitLab; {permission} is read registry, {link} links GitLab's token page, {path} is GitLab's menu path",
  },
  "registryCredentials.helpDocker": {
    message:
      "Docker Hub 사용자 이름과 비밀번호를 입력하거나, {link} {path}에서 {permission} 액세스 권한이 있는 액세스 토큰을 만들어 비밀번호 대신 사용하세요.",
    description:
      "Help under the Registry select for Docker; {permission} is read, {link} links Docker Hub's security settings, {path} is Docker Hub's menu path",
  },
  "registryCredentials.helpGoogle": {
    message:
      "{link}에서 {permission} 권한이 있는 서비스 계정을 만들어야 합니다.",
    description:
      "Help under the Registry select for Google Artifact; {permission} is the reader role, {link} links Google Cloud's service account page",
  },
  "registryCredentials.helpEcr": {
    message:
      "사용자 이름은 ECR 저장소를 소유한 AWS 계정 ID입니다. 개인 액세스 토큰은 {command} 명령을 실행해 얻습니다. 자세한 내용은 {link}를 참고하세요.",
    description:
      "Help under the Registry select for AWS ECR; {command} is the aws CLI command, {link} is helpEcrLink",
  },
  "registryCredentials.helpEcrLink": {
    message: "AWS 문서",
    description: "Link text to the AWS ECR authentication docs inside helpEcr",
  },
  "registryCredentials.fieldRegistryImmutable": {
    message: "레지스트리는 생성 후 변경할 수 없습니다.",
    description: "Hint under the read-only Registry in the edit dialog",
  },
};

export default koRegistryCredentials;
