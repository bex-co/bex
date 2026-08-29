import enCommon from "@/common/locales/en";
import enAuth from "@/features/auth/locales/en";
import enLogs from "@/features/logs/locales/en";
import enMetrics from "@/features/metrics/locales/en";
import enServices from "@/features/services/locales/en";
import enDatabases from "@/features/databases/locales/en";
import enKeyValue from "@/features/keyvalue/locales/en";
import enApiKeys from "@/features/api-keys/locales/en";
import enWorkspaces from "@/features/workspaces/locales/en";
import enTeam from "@/features/team/locales/en";
import enUsage from "@/features/usage/locales/en";
import enAudit from "@/features/audit/locales/en";
import enGit from "@/features/git/locales/en";
import enNotifications from "@/features/notifications/locales/en";
import enProjects from "@/features/projects/locales/en";
import enEnvironments from "@/features/environments/locales/en";
import enRegistryCredentials from "@/features/registry-credentials/locales/en";
import enConnectedAgents from "@/features/connected-agents/locales/en";
import enSessions from "@/features/sessions/locales/en";
import enBlueprints from "@/features/blueprints/locales/en";
import enEnvGroups from "@/features/env-groups/locales/en";
import enWebhooks from "@/features/webhooks/locales/en";
import enDeploys from "@/features/deploys/locales/en";
import enSSHKeys from "@/features/ssh-keys/locales/en";
import enInvites from "@/features/invites/locales/en";
import enAgentSessions from "@/features/agent-sessions/locales/en";
import enCapabilities from "@/features/capabilities/locales/en";
import enOnboarding from "@/features/onboarding/locales/en";
import {
  DEFAULT_LANGUAGE,
  type SupportedLanguage,
  type TranslationEntry,
} from "./config";

export type { SupportedLanguage, TranslationEntry } from "./config";
export {
  SUPPORTED_LANGUAGES,
  LANGUAGE_NAMES,
  DEFAULT_LANGUAGE,
} from "./config";
export { persistLanguage } from "./utils";
export { detectLanguage } from "./detect-language";

export function extractMessages(
  obj: Record<string, TranslationEntry>,
): Record<string, string> {
  return Object.fromEntries(
    Object.entries(obj).map(([key, entry]) => [key, entry.message]),
  );
}

/**
 * Per-language flattened `{ key: message }` resources, aggregated from every
 * feature's `locales/<lang>.ts`. Adding a language: create `<lang>.ts` next to
 * each `en.ts` above, add it to `SUPPORTED_LANGUAGES`/`LANGUAGE_NAMES`
 * (config.ts), then register its imports here.
 */
export const en: Record<string, string> = {
  ...extractMessages(enCommon),
  ...extractMessages(enAuth),
  ...extractMessages(enLogs),
  ...extractMessages(enMetrics),
  ...extractMessages(enServices),
  ...extractMessages(enDatabases),
  ...extractMessages(enKeyValue),
  ...extractMessages(enApiKeys),
  ...extractMessages(enWorkspaces),
  ...extractMessages(enTeam),
  ...extractMessages(enUsage),
  ...extractMessages(enAudit),
  ...extractMessages(enGit),
  ...extractMessages(enNotifications),
  ...extractMessages(enProjects),
  ...extractMessages(enEnvironments),
  ...extractMessages(enRegistryCredentials),
  ...extractMessages(enConnectedAgents),
  ...extractMessages(enSessions),
  ...extractMessages(enBlueprints),
  ...extractMessages(enEnvGroups),
  ...extractMessages(enWebhooks),
  ...extractMessages(enDeploys),
  ...extractMessages(enSSHKeys),
  ...extractMessages(enInvites),
  ...extractMessages(enAgentSessions),
  ...extractMessages(enCapabilities),
  ...extractMessages(enOnboarding),
};

// Only the default language is bundled eagerly (into the entry chunk). Other
// locales are pulled on demand via `loadLanguageResources` (w9/m60 t003) so an
// English session never ships the Chinese catalog and per-locale weight scales
// as languages are added.
export const resources: Partial<
  Record<SupportedLanguage, { translation: Record<string, string> }>
> = {
  en: { translation: en },
};

/**
 * Resolve a language's flattened `{ key: message }` catalog. The default
 * language is already in the bundle; any other is a lazy `import()` (its own
 * async client chunk, and a synchronous server-side require during SSR). Extend
 * the switch when adding a language whose namespaces live in a `resources-<lang>.ts`.
 */
export async function loadLanguageResources(
  lang: SupportedLanguage,
): Promise<Record<string, string>> {
  if (lang === DEFAULT_LANGUAGE) return en;
  switch (lang) {
    case "zh":
      return (await import("./resources-zh")).default;
    default:
      return en;
  }
}
