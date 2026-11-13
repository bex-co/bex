import type { Translate } from "@/common/hooks/use-translations";
import { en } from "@/i18n";

type WireParams = ReadonlyArray<{ name: string; value: string }>;

/**
 * One sentence of a reason's hint or cause (w5/m356), with bex-api's English
 * for a code the dashboard does not word.
 */
export interface DeployReasonPart {
  code: string;
  params: Record<string, string>;
  text: string;
}

/**
 * A deploy reason sentence by its stable code (bex-api's DeployReason,
 * w4/566): the operator and bex-api word it in English for every API, and the
 * dashboard words it in its own language when it knows the code.
 */
export interface DeployReasonTemplate {
  code: string;
  params: Record<string, string>;
  /** The sentences of its hint or cause, in order (w5/m356). */
  parts: DeployReasonPart[];
  /** The tail from the first sentence no template words, as bex-api wrote it. */
  remainder: string;
}

const paramsOf = (params: WireParams) =>
  Object.fromEntries(params.map((p) => [p.name, p.value]));

/** The wire DeployReason as a view, or null. */
export function toDeployReasonTemplate(
  raw:
    | {
        code: string;
        params: WireParams;
        parts: ReadonlyArray<{
          code: string;
          params: WireParams;
          text: string;
        }>;
        remainder: string;
      }
    | null
    | undefined,
): DeployReasonTemplate | null {
  if (!raw) return null;
  return {
    code: raw.code,
    params: paramsOf(raw.params),
    parts: raw.parts.map((p) => ({
      code: p.code,
      params: paramsOf(p.params),
      text: p.text,
    })),
    remainder: raw.remainder,
  };
}

/**
 * A Go duration (restart_wait_timed_out's {wait}: "3h0m0s", "1h30m0s") in
 * words. en's message is the Go string itself, so an en reader sees bex-api's
 * sentence; a value in any other shape reads as-is.
 */
function durationText(t: Translate, value: string): string {
  const m = /^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+(?:\.\d+)?)s)?$/.exec(value);
  if (!m) return value;
  const [, hours, minutes, seconds] = m;
  const units = [
    Number(hours) > 0 ? t("deploys.reasonWait.hours", { hours }) : "",
    Number(minutes) > 0 ? t("deploys.reasonWait.minutes", { minutes }) : "",
    Number(seconds) > 0 ? t("deploys.reasonWait.seconds", { seconds }) : "",
  ].filter(Boolean);
  if (units.length === 0) return value;
  return t("deploys.reasonWait.duration", {
    wait: value,
    units: units.join(" "),
  });
}

/**
 * One sentence in the dashboard's language, or null when the catalog does
 * not word its code. A parameter that is itself an English word the catalog
 * words (`deploys.reasonParam.<name>.<value>`, a probe's "startup") is worded
 * too, and {wait} reads as a duration; any other value, an id or a path,
 * reads as-is.
 */
function wordSentence(
  t: Translate,
  code: string,
  params: Record<string, string>,
): string | null {
  const key = `deploys.reason.${code}`;
  if (!(key in en)) return null;
  const worded = Object.fromEntries(
    Object.entries(params).map(([name, value]) => {
      if (name === "wait") return [name, durationText(t, value)];
      const word = `deploys.reasonParam.${name}.${value}`;
      return [name, word in en ? t(word as keyof typeof en) : value];
    }),
  );
  return t(key as keyof typeof en, worded);
}

/**
 * The reason in the dashboard's language when its code is one the dashboard
 * words, else bex-api's own sentence. A hint or cause bex-api read as parts
 * (w5/m356) follows the sentence part by part, each worded or in its own
 * English, then any remainder as bex-api wrote it.
 */
export function deployReasonText(
  t: Translate,
  text: string,
  template: DeployReasonTemplate | null | undefined,
): string {
  if (!template) return text;
  const tail = template.parts.length > 0 || template.remainder !== "";
  const params = Object.fromEntries(
    Object.entries(template.params).map(([name, value]) => [
      name,
      tail && (name === "hint" || name === "cause") ? "" : value,
    ]),
  );
  const sentence = wordSentence(t, template.code, params);
  if (sentence === null) return text;
  if (!tail) return sentence;
  const parts = template.parts.map(
    (p) => wordSentence(t, p.code, p.params) ?? p.text,
  );
  return sentence + parts.join("") + template.remainder;
}
