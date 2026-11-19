// The sandbox template a session runs on (w10/m519): display helpers shared by
// the sandbox picker, the session header, the automation form and Settings →
// Agents. Every name and size comes from bex-api's capabilities or environment
// read; nothing here names a template or a size.

import type { Translate } from "@/common/hooks/use-translations";
import { dashboardLanguage } from "@/common/lib/format/language";
import { LANGUAGE_TAGS } from "@/i18n/config";
import type { AgentSandboxTemplateView } from "@/features/agent-sessions/types";

/** The size fields of a template, as both the capabilities and environment reads carry them. */
type TemplateSize = Pick<AgentSandboxTemplateView, "cpu" | "memoryGb">;

// One formatter per language: labels are drawn on every render, and the
// session header re-renders each second while its clock ticks.
const sizeFormats = new Map<string, Intl.NumberFormat>();
function sizeFormat(language: string | undefined): Intl.NumberFormat {
  const tag = LANGUAGE_TAGS[dashboardLanguage(language)];
  let format = sizeFormats.get(tag);
  if (!format) {
    format = new Intl.NumberFormat(tag, { maximumFractionDigits: 2 });
    sizeFormats.set(tag, format);
  }
  return format;
}

/** A template's CPU and memory ("4 CPU, 8 GB"), numbers in the dashboard language. */
function templateSize(
  template: TemplateSize,
  t: Translate,
  language: string | undefined,
): string {
  const number = sizeFormat(language);
  return t("agentSessions.sandboxSize", {
    cpu: number.format(template.cpu),
    memory: number.format(template.memoryGb),
  });
}

/** A template's name and size ("agent-4c-8g · 4 CPU, 8 GB"). */
export function templateLabel(
  template: TemplateSize & { name: string },
  t: Translate,
  language: string | undefined,
): string {
  return t("agentSessions.sandboxOption", {
    name: template.name,
    size: templateSize(template, t, language),
  });
}

/**
 * The label of the template named `name`: its name and size when the list
 * carries it, else the name alone (a template retired since it was chosen).
 */
export function namedTemplateLabel(
  name: string,
  templates: readonly (TemplateSize & { name: string })[],
  t: Translate,
  language: string | undefined,
): string {
  const template = templates.find((candidate) => candidate.name === name);
  return template ? templateLabel(template, t, language) : name;
}
