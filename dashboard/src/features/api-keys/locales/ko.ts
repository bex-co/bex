import type { TranslationEntry } from "@/i18n";

const koApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API 키",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "스크립트와 에이전트를 위한 머신 자격 증명입니다. 워크스페이스 전체에서 공유되며, 키를 관리할 수 있는 사람은 본인 키뿐 아니라 모든 키를 볼 수 있습니다.",
    description: "Settings API Keys section card description",
  },
  "apiKeys.cliGuide": {
    message: "CLI 설정하기.",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.colName": {
    message: "이름",
    description: "API Keys table column header",
  },
  "apiKeys.colClientId": {
    message: "클라이언트 ID",
    description:
      "API Keys table column header — OAuth2 client_id paired with the secret",
  },
  "apiKeys.colCreated": {
    message: "생성일",
    description: "API Keys table column header",
  },
  "apiKeys.colCreatedBy": {
    message: "만든 사람",
    description: "API Keys table column header — who minted the key",
  },
  "apiKeys.colLastUsed": {
    message: "마지막 사용",
    description:
      "API Keys table column header — when a token for the key was last used",
  },
  "apiKeys.neverUsed": {
    message: "사용 안 함",
    description: "API Keys last-used cell when the key has never been used",
  },
  "apiKeys.emptyTitle": {
    message: "API 키가 없습니다",
    description: "API Keys empty-state title",
  },
  "apiKeys.emptyBody": {
    message: "스크립트나 에이전트를 인증하려면 키를 만드세요.",
    description: "API Keys empty-state body",
  },
  "apiKeys.forbiddenTitle": {
    message: "권한이 없습니다",
    description: "API Keys state when the caller lacks permission (403)",
  },
  "apiKeys.forbiddenBody": {
    message: "이 워크스페이스의 API 키를 관리할 권한이 없습니다.",
    description: "API Keys forbidden-state body",
  },
  "apiKeys.errorTitle": {
    message: "API 키를 불러오지 못했습니다",
    description: "API Keys generic error title",
  },
  "apiKeys.errorBody": {
    message: "문제가 발생했습니다. 다시 시도하세요.",
    description: "API Keys generic error body",
  },
  "apiKeys.create": {
    message: "API 키 만들기",
    description: "Button that opens the mint dialog",
  },
  "apiKeys.createTitle": {
    message: "API 키 만들기",
    description: "Mint dialog title (name step)",
  },
  "apiKeys.createDescription": {
    message: "나중에 알아볼 수 있도록 키 이름을 지정하세요.",
    description: "Mint dialog description (name step)",
  },
  "apiKeys.fieldName": {
    message: "이름",
    description: "Mint dialog name field label",
  },
  "apiKeys.fieldNamePlaceholder": {
    message: "예: deploy-agent",
    description: "Mint dialog name field placeholder",
  },
  "apiKeys.createCancel": {
    message: "취소",
    description: "Mint dialog cancel button (name step)",
  },
  "apiKeys.createSubmit": {
    message: "만들기",
    description: "Mint dialog submit button (name step)",
  },
  "apiKeys.createdTitle": {
    message: "API 키를 만들었습니다",
    description: "Mint dialog title (secret-shown step)",
  },
  "apiKeys.createdWarning": {
    message:
      "지금 클라이언트 ID와 시크릿을 복사하세요. 시크릿은 다시 볼 수 없습니다.",
    description: "Mint dialog warning (secret-shown step)",
  },
  "apiKeys.fieldClientId": {
    message: "클라이언트 ID",
    description: "Mint dialog label for the OAuth2 client_id",
  },
  "apiKeys.fieldSecret": {
    message: "클라이언트 시크릿",
    description: "Mint dialog label for the one-time client_secret",
  },
  "apiKeys.copyClientId": {
    message: "클라이언트 ID 복사",
    description: "Copy button aria-label for the client_id",
  },
  "apiKeys.copyClientIdNamed": {
    message: "{name}의 클라이언트 ID 복사",
    description: "Copy button aria-label for one API client row's client_id",
  },
  "apiKeys.copySecret": {
    message: "클라이언트 시크릿 복사",
    description: "Copy button aria-label for the client_secret",
  },
  "apiKeys.exchangeHelp": {
    message:
      "이것은 bearer 토큰이 아니라 OAuth2 client_credentials 클라이언트입니다. {tokenEndpoint}에서 교환하세요:\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nscope는 보내지 마세요. 이 키는 연결 문자열을 포함해 이 워크스페이스에서 모든 권한을 가지므로 비밀번호처럼 다루세요. 반환된 access_token을 Authorization: Bearer로 사용하세요.",
    description:
      "Mint dialog usage copy for the client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "완료",
    description: "Mint dialog close button (secret-shown step)",
  },
  "apiKeys.copy": {
    message: "시크릿 복사",
    description: "Copy-to-clipboard icon button label",
  },
  "apiKeys.copied": {
    message: "클립보드에 복사했습니다",
    description: "Toast on a successful secret copy",
  },
  "apiKeys.copyError": {
    message: "클립보드에 복사하지 못했습니다",
    description: "Toast on a failed secret copy",
  },
  "apiKeys.createSuccess": {
    message: "{name} 키를 만들었습니다",
    description: "Toast on a successful mint",
  },
  "apiKeys.createError": {
    message: "{name} 키를 만들지 못했습니다",
    description: "Toast on a failed mint",
  },
  "apiKeys.revoke": {
    message: "취소",
    description: "Row action / confirmation button to revoke a key",
  },
  "apiKeys.revokeNamed": {
    message: "API 키 {name} 취소",
    description: "Row revoke button aria-label identifying the API key",
  },
  "apiKeys.revokeConfirmTitle": {
    message: "{name} 키를 취소하시겠습니까?",
    description: "Revoke-confirmation dialog title",
  },
  "apiKeys.revokeConfirmBody": {
    message:
      "이 키로 인증하는 모든 항목이 즉시 작동을 멈춥니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Revoke-confirmation dialog body",
  },
  "apiKeys.revokeCancel": {
    message: "취소",
    description: "Revoke-confirmation dialog cancel button",
  },
  "apiKeys.revokeSuccess": {
    message: "{name} 키를 취소했습니다",
    description: "Toast on a successful revoke",
  },
  "apiKeys.revokeError": {
    message: "{name} 키를 취소하지 못했습니다",
    description: "Toast on a failed revoke",
  },
};

export default koApiKeys;
