import type { TranslationEntry } from "@/i18n";

const ja: Record<string, TranslationEntry> = {
  "connectedAgents.title": {
    message: "接続済みのエージェント",
    description: "Connected Agents settings card title",
  },
  "connectedAgents.description": {
    message:
      "あなたに代わって操作することを承認した OAuth クライアントです。失効すると、そのアクセストークンは直ちに無効になります。",
    description: "Connected Agents settings card description",
  },
  "connectedAgents.colClient": {
    message: "クライアント",
    description: "Connected Agents table column: client name",
  },
  "connectedAgents.colScopes": {
    message: "スコープ",
    description: "Connected Agents table column: granted scopes",
  },
  "connectedAgents.colGranted": {
    message: "付与日時",
    description: "Connected Agents table column: grant date",
  },
  "connectedAgents.emptyTitle": {
    message: "接続済みのエージェントはありません",
    description: "Connected Agents empty state title",
  },
  "connectedAgents.emptyBody": {
    message: "承認した OAuth クライアントはまだありません。",
    description: "Connected Agents empty state body",
  },
  "connectedAgents.errorTitle": {
    message: "接続済みのエージェントを読み込めませんでした",
    description: "Connected Agents generic error state title",
  },
  "connectedAgents.errorBody": {
    message: "問題が発生しました。しばらくしてから、もう一度お試しください。",
    description: "Connected Agents generic error state body",
  },
  "connectedAgents.revoke": {
    message: "失効",
    description: "Connected Agents row revoke confirmation button",
  },
  "connectedAgents.revokeNamed": {
    message: "接続済みのエージェント {name} を失効",
    description:
      "Connected Agents row revoke button aria-label identifying the client",
  },
  "connectedAgents.revokeConfirmTitle": {
    message: "「{name}」のアクセスを失効しますか？",
    description: "Connected Agents revoke confirmation dialog title",
  },
  "connectedAgents.revokeConfirmBody": {
    message:
      "このクライアントが保持しているすべてのアクセストークンが、直ちに無効になります。再びあなたとして操作するには、再承認が必要です。",
    description: "Connected Agents revoke confirmation dialog body",
  },
  "connectedAgents.revokeCancel": {
    message: "キャンセル",
    description: "Connected Agents revoke confirmation dialog cancel button",
  },
  "connectedAgents.revokeSuccess": {
    message: "「{name}」のアクセスを失効しました",
    description: "Connected Agents revoke success toast",
  },
  "connectedAgents.revokeError": {
    message: "「{name}」のアクセスを失効できませんでした",
    description: "Connected Agents revoke failure toast",
  },
};

export default ja;
