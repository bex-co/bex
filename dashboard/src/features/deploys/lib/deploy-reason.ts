import type { Translate } from "@/common/hooks/use-translations";
import { en } from "@/i18n";

/**
 * A deploy reason sentence by its stable code (bex-api's DeployReason,
 * w4/566): the operator and bex-api word it in English for every API, and the
 * dashboard words it in its own language when it knows the code.
 */
export interface DeployReasonTemplate {
  code: string;
  params: Record<string, string>;
}

/** The wire DeployReason as a view, or null. */
export function toDeployReasonTemplate(
  raw:
    | {
        code: string;
        params: ReadonlyArray<{ name: string; value: string }>;
      }
    | null
    | undefined,
): DeployReasonTemplate | null {
  if (!raw) return null;
  return {
    code: raw.code,
    params: Object.fromEntries(raw.params.map((p) => [p.name, p.value])),
  };
}

/**
 * The reason in the dashboard's language when its code is one the dashboard
 * words, else bex-api's own sentence.
 */
export function deployReasonText(
  t: Translate,
  text: string,
  template: DeployReasonTemplate | null | undefined,
): string {
  const key = template ? `deploys.reason.${template.code}` : "";
  if (!template || !(key in en)) return text;
  return t(key as keyof typeof en, template.params);
}
