import type { TranslationEntry } from "@/i18n";

const ko: Record<string, TranslationEntry> = {
  "connectedAgents.title": {
    message: "연결된 에이전트",
    description: "Connected Agents settings card title",
  },
  "connectedAgents.description": {
    message:
      "회원님을 대신하여 작업하도록 승인한 OAuth 클라이언트입니다. 클라이언트를 취소하면 해당 액세스 토큰이 즉시 무효화됩니다.",
    description: "Connected Agents settings card description",
  },
  "connectedAgents.colClient": {
    message: "클라이언트",
    description: "Connected Agents table column: client name",
  },
  "connectedAgents.colScopes": {
    message: "스코프",
    description: "Connected Agents table column: granted scopes",
  },
  "connectedAgents.colGranted": {
    message: "승인일",
    description: "Connected Agents table column: grant date",
  },
  "connectedAgents.emptyTitle": {
    message: "연결된 에이전트가 없습니다",
    description: "Connected Agents empty state title",
  },
  "connectedAgents.emptyBody": {
    message: "아직 승인한 OAuth 클라이언트가 없습니다.",
    description: "Connected Agents empty state body",
  },
  "connectedAgents.errorTitle": {
    message: "연결된 에이전트를 불러오지 못했습니다",
    description: "Connected Agents generic error state title",
  },
  "connectedAgents.errorBody": {
    message: "문제가 발생했습니다. 잠시 후 다시 시도하세요.",
    description: "Connected Agents generic error state body",
  },
  "connectedAgents.revoke": {
    message: "취소",
    description: "Connected Agents row revoke confirmation button",
  },
  "connectedAgents.revokeNamed": {
    message: "연결된 에이전트 {name} 취소",
    description:
      "Connected Agents row revoke button aria-label identifying the client",
  },
  "connectedAgents.revokeConfirmTitle": {
    message: '"{name}" 접근 권한을 취소하시겠습니까?',
    description: "Connected Agents revoke confirmation dialog title",
  },
  "connectedAgents.revokeConfirmBody": {
    message:
      "이 클라이언트가 가진 모든 액세스 토큰이 즉시 무효화됩니다. 다시 회원님으로 작업하려면 재승인이 필요합니다.",
    description: "Connected Agents revoke confirmation dialog body",
  },
  "connectedAgents.revokeCancel": {
    message: "취소",
    description: "Connected Agents revoke confirmation dialog cancel button",
  },
  "connectedAgents.revokeSuccess": {
    message: '"{name}" 접근 권한을 취소했습니다',
    description: "Connected Agents revoke success toast",
  },
  "connectedAgents.revokeError": {
    message: '"{name}" 접근 권한을 취소하지 못했습니다',
    description: "Connected Agents revoke failure toast",
  },
};

export default ko;
