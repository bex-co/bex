import { dashboardLanguage } from "@/common/lib/format/language";
import { DEFAULT_LANGUAGE } from "@/i18n/config";

/**
 * The agent model section of the bex.co Terms of Service (ADR091 D8), in the
 * reader's language: bex.co serves every dashboard language.
 */
export function agentModelTermsUrl(language: string | undefined): string {
  const lang = dashboardLanguage(language);
  const prefix = lang === DEFAULT_LANGUAGE ? "" : `/${lang}`;
  return `https://bex.co${prefix}/terms-of-service#agent-model`;
}

export const AGENT_MODEL_TERMS_URL = agentModelTermsUrl(DEFAULT_LANGUAGE);
