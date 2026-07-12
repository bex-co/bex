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
import zhCommon from "@/common/locales/zh";
import zhAuth from "@/features/auth/locales/zh";
import zhLogs from "@/features/logs/locales/zh";
import zhMetrics from "@/features/metrics/locales/zh";
import zhServices from "@/features/services/locales/zh";
import zhDatabases from "@/features/databases/locales/zh";
import zhKeyValue from "@/features/keyvalue/locales/zh";
import zhApiKeys from "@/features/api-keys/locales/zh";
import zhWorkspaces from "@/features/workspaces/locales/zh";
import zhTeam from "@/features/team/locales/zh";
import zhUsage from "@/features/usage/locales/zh";
import zhAudit from "@/features/audit/locales/zh";
import type { SupportedLanguage, TranslationEntry } from "./config";

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
};

export const zh: Record<string, string> = {
  ...extractMessages(zhCommon),
  ...extractMessages(zhAuth),
  ...extractMessages(zhLogs),
  ...extractMessages(zhMetrics),
  ...extractMessages(zhServices),
  ...extractMessages(zhDatabases),
  ...extractMessages(zhKeyValue),
  ...extractMessages(zhApiKeys),
  ...extractMessages(zhWorkspaces),
  ...extractMessages(zhTeam),
  ...extractMessages(zhUsage),
  ...extractMessages(zhAudit),
};

export const resources: Record<
  SupportedLanguage,
  { translation: Record<string, string> }
> = {
  en: { translation: en },
  zh: { translation: zh },
};
