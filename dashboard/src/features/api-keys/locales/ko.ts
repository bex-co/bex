import type { TranslationEntry } from "@/i18n";

const koApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API 키",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "스크립트와 에이전트를 위한 머신 자격 증명입니다. 각 키는 만든 사람의 현재 멤버 역할로, 이 워크스페이스에서만 동작합니다. 워크스페이스 전체에서 공유되며, 키를 관리할 수 있는 사람은 본인 키뿐 아니라 모든 키를 볼 수 있습니다.",
    description:
      "Settings API Keys section card description: what a key is, that it acts with its creator's current role in this workspace, and that the list is shared",
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
      "API Keys table column header: the key's id, which is also its OAuth2 client_id",
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
    description: "API Keys table column header: when the key was last used",
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
    description: "Mint dialog title (key-shown step)",
  },
  "apiKeys.createdWarning": {
    message: "지금 키를 복사하세요. 다시 볼 수 없습니다.",
    description:
      "Mint dialog warning (key-shown step): the key is shown only once",
  },
  "apiKeys.fieldKey": {
    message: "API 키",
    description:
      "Mint dialog label for the key itself, the one string sent as Authorization: Bearer",
  },
  "apiKeys.copyKey": {
    message: "API 키 복사",
    description: "Copy button aria-label for the key",
  },
  "apiKeys.keyHelp": {
    message:
      "모든 요청에 Authorization: Bearer로 보내세요. 이 키는 이 워크스페이스에서 회원님의 현재 멤버 역할로 동작하고 역할이 바뀌면 그에 따르므로 비밀번호처럼 다루세요.",
    description:
      "Mint dialog copy under the key: how to send it, and that it acts with the creator's current member role in this workspace and follows it",
  },
  "apiKeys.curlTitle": {
    message: "요청 예시",
    description:
      "Mint dialog label for the curl example that lists services with the key",
  },
  "apiKeys.copyCurl": {
    message: "요청 예시 복사",
    description: "Copy button aria-label for the curl example",
  },
  "apiKeys.oauthDetails": {
    message: "OAuth 클라이언트 자격 증명",
    description:
      "Mint dialog title of the collapsed detail holding the client ID, the client secret and the optional token exchange",
  },
  "apiKeys.fieldClientId": {
    message: "클라이언트 ID",
    description:
      "Mint dialog label for the OAuth2 client_id, inside the OAuth client credentials detail",
  },
  "apiKeys.fieldSecret": {
    message: "클라이언트 시크릿",
    description:
      "Mint dialog label for the one-time client_secret, inside the OAuth client credentials detail",
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
      "이 키는 OAuth2 클라이언트로도 사용할 수 있습니다. 수명이 짧은 토큰을 선호한다면 {tokenEndpoint}에서 클라이언트 ID와 클라이언트 시크릿을 액세스 토큰으로 교환하세요:\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nscope는 보내지 마세요. 반환된 access_token을 키 대신 Authorization: Bearer로 사용하세요.",
    description:
      "Mint dialog copy inside the OAuth client credentials detail: the optional client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "완료",
    description: "Mint dialog close button (key-shown step)",
  },
  "apiKeys.copied": {
    message: "클립보드에 복사했습니다",
    description:
      "Toast on a successful copy (the key, the example request, the client ID or the secret)",
  },
  "apiKeys.copyError": {
    message: "클립보드에 복사하지 못했습니다",
    description: "Toast on a failed copy",
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
