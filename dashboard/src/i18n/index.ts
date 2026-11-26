import routerMessages from "@/features/router/locales/en";
import enCommon from "@/common/locales/en";
import enAuth from "@/features/auth/locales/en";
import enLogs from "@/features/logs/locales/en";
import enMetrics from "@/features/metrics/locales/en";
import enServices from "@/features/services/locales/en";
import enDatabases from "@/features/databases/locales/en";
import enKeyValue from "@/features/keyvalue/locales/en";
import enApiKeys from "@/features/api-keys/locales/en";
import enWorkspaces from "@/features/workspaces/locales/en";
import enAvatars from "@/features/avatars/locales/en";
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
import enAgentEnvironment from "@/features/agent-environment/locales/en";
import enAgentAutomations from "@/features/agent-automations/locales/en";
import enSandboxGroups from "@/features/sandbox-groups/locales/en";
import enBuildSources from "@/features/build-sources/locales/en";
import enLegal from "@/features/legal/locales/en";
import enOrganizations from "@/features/organizations/locales/en";
// Ory's own words for the loading frames of its cards, not the dashboard's
// (features/auth/ory-copy/keys.ts).
import enOryCopy from "@/features/auth/ory-copy/en";
import type { SupportedLanguage, TranslationEntry } from "./config";

export type { SupportedLanguage, TranslationEntry } from "./config";
export {
  SUPPORTED_LANGUAGES,
  LANGUAGE_NAMES,
  LANGUAGE_TAGS,
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
 * each `en.ts` above, add it to `SUPPORTED_LANGUAGES`/`LANGUAGE_NAMES`/
 * `LANGUAGE_TAGS` (config.ts), add a `resources-<lang>.ts` aggregating them,
 * then register its loader in `LAZY_RESOURCES` below.
 */
export const en: Record<string, string> = {
  ...extractMessages(enCommon),
  ...extractMessages(routerMessages),
  ...extractMessages(enAuth),
  ...extractMessages(enLogs),
  ...extractMessages(enMetrics),
  ...extractMessages(enServices),
  ...extractMessages(enDatabases),
  ...extractMessages(enKeyValue),
  ...extractMessages(enApiKeys),
  ...extractMessages(enWorkspaces),
  ...extractMessages(enAvatars),
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
  ...extractMessages(enAgentEnvironment),
  ...extractMessages(enAgentAutomations),
  ...extractMessages(enSandboxGroups),
  ...extractMessages(enBuildSources),
  ...extractMessages(enLegal),
  ...extractMessages(enOrganizations),
  ...enOryCopy,
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
 * async client chunk, and a synchronous server-side require during SSR). Each
 * `import()` must stay a literal path so the bundler can split it into its own
 * chunk; the `Record` makes adding a language without its loader a type error.
 */
const LAZY_RESOURCES: Record<
  Exclude<SupportedLanguage, "en">,
  () => Promise<{ default: Record<string, string> }>
> = {
  zh: () => import("./resources-zh"),
  ko: () => import("./resources-ko"),
  ja: () => import("./resources-ja"),
  es: () => import("./resources-es"),
  pt: () => import("./resources-pt"),
  ru: () => import("./resources-ru"),
  de: () => import("./resources-de"),
};

export async function loadLanguageResources(
  lang: SupportedLanguage,
): Promise<Record<string, string>> {
  // en is DEFAULT_LANGUAGE, the catalog already in the entry bundle.
  if (lang === "en") return en;
  return (await LAZY_RESOURCES[lang]()).default;
}
