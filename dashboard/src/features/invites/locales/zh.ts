import type { TranslationEntry } from "@/i18n";

const zhInvites: Record<string, TranslationEntry> = {
  "invites.openingTitle": {
    message: "正在打开邀请",
    description: "Invite fallback page title while routing into authentication",
  },
  "invites.openingDescription": {
    message: "正在安全地跳转到 bex…",
    description: "Invite fallback page progress description",
  },
};

export default zhInvites;
